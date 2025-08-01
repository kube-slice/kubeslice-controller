/*
 * 	Copyright (c) 2022 Avesha, Inc. All rights reserved. # # SPDX-License-Identifier: Apache-2.0
 *
 * 	Licensed under the Apache License, Version 2.0 (the "License");
 * 	you may not use this file except in compliance with the License.
 * 	You may obtain a copy of the License at
 *
 * 	http://www.apache.org/licenses/LICENSE-2.0
 *
 * 	Unless required by applicable law or agreed to in writing, software
 * 	distributed under the License is distributed on an "AS IS" BASIS,
 * 	WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * 	See the License for the specific language governing permissions and
 * 	limitations under the License.
 */

package service

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kubeslice/kubeslice-controller/metrics"

	workerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/worker/v1alpha1"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"

	controllerv1alpha1 "github.com/kubeslice/kubeslice-controller/apis/controller/v1alpha1"
	ossEvents "github.com/kubeslice/kubeslice-controller/events"
	metricMock "github.com/kubeslice/kubeslice-controller/metrics/mocks"
	"github.com/kubeslice/kubeslice-controller/service/mocks"
	"github.com/kubeslice/kubeslice-controller/util"
	utilmock "github.com/kubeslice/kubeslice-controller/util/mocks"

	"github.com/dailymotion/allure-go"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	kubeerrors "k8s.io/apimachinery/pkg/api/errors"
	kubemachine "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestClusterSuite(t *testing.T) {
	for k, v := range ClusterTestbed {
		t.Run(k, func(t *testing.T) {
			allure.Test(t, allure.Name(k),
				allure.Action(func() {
					v(t)
				}))
		})
	}
}

var ClusterTestbed = map[string]func(*testing.T){
	"TestReconcileClusterClusterNotFound":                                   testReconcileClusterClusterNotFound,
	"TestReconcileClusterProjectNamespaceNotFound":                          testReconcileClusterProjectNamespaceNotFound,
	"TestReconcileClusterDeletionClusterFail":                               testReconcileClusterDeletionClusterFail,
	"TestReconcileClusterSecretNotFound":                                    testReconcileClusterSecretNotFound,
	"TestDeleteClustersListFail":                                            testDeleteClustersListFail,
	"TestDeleteClusterDeleteFail":                                           testDeleteClusterDeleteFail,
	"TestDeleteClusterDeletePass":                                           testDeleteClusterDeletePass,
	"TestClusterPass":                                                       testClusterPass,
	"TestClusterResetNodeIPsIfEmptyString":                                  testClusterResetNodeIPsIfEmptyString,
	"TestReconcileClusterUpdateSecretFail":                                  testReconcileClusterUpdateSecretFail,
	"TestReconcileClusterServiceAccountSecretNil":                           testReconcileClusterServiceAccountSecretNil,
	"TestReconcileClusterDeletionRaisesEvent":                               testReconcileClusterDeletionRaisesEvent,
	"TestReconcileClusterDeletionRequeueForDeregister":                      testReconcileClusterDeletionRequeueForDeregister,
	"TestReconcileClusterDeletionSuccessAfterWorkerFailedToRemoveFinalizer": testReconcileClusterDeletionSuccessAfterWorkerFailedToRemoveFinalizer,
	"TestReconcileClusterDeletionFailureAfterWorkerFailedToRemoveFinalizer": testReconcileClusterDeletionFailureAfterWorkerFailedToRemoveFinalizer,
	"TestReconcileClusterDeletionDeregisterFailed":                          testReconcileClusterDeletionDeregisterFailed,
	"TestReconcileClusterDeletionDeregisterSuccess":                         testReconcileClusterDeletionDeregisterSuccess,
}

func testReconcileClusterClusterNotFound(t *testing.T) {
	// fine
	// requeue must be false and error nil
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
	}
	clusterName := types.NamespacedName{} // passing empty clusterName(name and namespace)
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(kubeerrors.NewNotFound(util.Resource("ClusterTest"), "cluster not found"))
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
}

func testReconcileClusterProjectNamespaceNotFound(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{} // passing empty clusterName(name and namespace)
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(kubeerrors.NewNotFound(util.Resource("ClusterTest"), "namespace not found"))
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionClusterFail(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	timeStamp := kubemachine.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// finaliser
	err := errors.New(" RemoveWorkerClusterServiceAccountAndRoleBindings internal error")
	acsService.On("RemoveWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.AnythingOfType("*v1alpha1.Cluster")).Return(ctrl.Result{}, nil)
	clientMock.On("Update", ctx, mock.Anything).Return(err).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	//	t.Error(result, err)
	require.False(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterSecretNotFound(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	//	secret := corev1.Secret{}
	serviceAccountSecretNamespacedName := types.NamespacedName{
		Namespace: requestObj.Namespace,
		Name:      "random",
	}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.Cluster")).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), &corev1.ServiceAccount{}).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.ServiceAccount)
		if arg.Secrets == nil {
			arg.Secrets = make([]corev1.ObjectReference, 1)
		}
		arg.Secrets[0].Name = "random"
	})
	acsService.On("ReconcileWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.Anything).Return(ctrl.Result{}, nil)
	clientMock.On("Get", ctx, serviceAccountSecretNamespacedName, mock.Anything).Return(kubeerrors.NewNotFound(util.Resource("ClusterTest"), " secret not found"))
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testDeleteClustersListFail(t *testing.T) {
	clusters := &controllerv1alpha1.ClusterList{}
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
	}
	clientMock := &utilmock.Client{}
	listerr := errors.New("list failed")
	namespace := "cisco"
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	clientMock.On("List", ctx, clusters, mock.Anything).Return(listerr)
	result, err := clusterService.DeleteClusters(ctx, namespace)
	require.False(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
}

func testDeleteClusterDeletePass(t *testing.T) {
	// clusters := &controllerv1alpha1.ClusterList{}
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}
	clientMock := &utilmock.Client{}
	namespace := "cisco"
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ClusterList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.Cluster, 1)
		}
		arg.Items[0].GenerateName = "random"
	})
	clientMock.On("Delete", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.DeleteClusters(ctx, namespace)
	require.Nil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testDeleteClusterDeleteFail(t *testing.T) {
	// clusters := &controllerv1alpha1.ClusterList{}
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}
	clientMock := &utilmock.Client{}
	deleteerr := errors.New("delete failed")
	namespace := "cisco"
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("List", ctx, mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(1).(*controllerv1alpha1.ClusterList)
		if arg.Items == nil {
			arg.Items = make([]controllerv1alpha1.Cluster, 1)
		}
		arg.Items[0].GenerateName = "random"
	})
	clientMock.On("Delete", ctx, mock.Anything).Return(deleteerr)
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.DeleteClusters(ctx, namespace)
	require.NotNil(t, err)
	require.False(t, result.Requeue)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func prepareTestContext(ctx context.Context, client client.Client,
	scheme *runtime.Scheme,
) context.Context {
	if scheme == nil {
		scheme = runtime.NewScheme()
	}
	controllerv1alpha1.AddToScheme(scheme)
	workerv1alpha1.AddToScheme(scheme)
	eventRecorder := events.NewEventRecorder(client, scheme, ossEvents.EventsMap, events.EventRecorderOptions{
		Version:   "v1alpha1",
		Cluster:   util.ClusterController,
		Component: util.ComponentController,
		Slice:     util.NotApplicable,
	})
	return util.PrepareKubeSliceControllersRequestContext(ctx, client, scheme, "ClusterTestController", &eventRecorder)
}

func testClusterPass(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	ssgService := &mocks.IWorkerSliceGatewayService{}
	mMock := &metricMock.IMetricRecorder{}

	clusterService := ClusterService{
		ns:   nsServiceMock,
		acs:  acsService,
		sgws: ssgService,
		mf:   mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	//	}
	nsResource := &corev1.Namespace{}

	secret := &corev1.Secret{}
	serviceAccountSecretNamespacedName := types.NamespacedName{
		Namespace: requestObj.Namespace,
		Name:      "random",
	}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.Cluster")).Return(nil).Once()
	serviceAccount := &corev1.ServiceAccount{} // Secrets: nil, //secret= nil should return requeue true and requeuetime>0
	clientMock.On("Get", ctx, mock.Anything, serviceAccount).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.ServiceAccount)
		if arg.Secrets == nil {
			arg.Secrets = make([]corev1.ObjectReference, 1)
			arg.Secrets[0].Name = "random"
		}
	}).Once()
	acsService.On("ReconcileWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.Anything).Return(ctrl.Result{}, nil)
	clientMock.On("Get", ctx, serviceAccountSecretNamespacedName, secret).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Secret)
		if arg.Data == nil {
			arg.Data = make(map[string][]byte, 2)
		}
	})

	clientMock.On("Status").Return(clientMock)
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Update", ctx, mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil)
	ssgService.On("NodeIpReconciliationOfWorkerSliceGateways", ctx, mock.Anything, requestObj.Namespace).Return(nil)
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testClusterResetNodeIPsIfEmptyString(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	ssgService := &mocks.IWorkerSliceGatewayService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:   nsServiceMock,
		acs:  acsService,
		sgws: ssgService,
		mf:   mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	//	}
	nsResource := &corev1.Namespace{}

	secret := &corev1.Secret{}
	serviceAccountSecretNamespacedName := types.NamespacedName{
		Namespace: requestObj.Namespace,
		Name:      "random",
	}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.Cluster")).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.Spec.NodeIPs = make([]string, 1)
		arg.Spec.NodeIPs[0] = ""
	}).Once()
	serviceAccount := &corev1.ServiceAccount{} // Secrets: nil, //secret= nil should return requeue true and requeuetime>0
	clientMock.On("Get", ctx, mock.Anything, serviceAccount).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.ServiceAccount)
		if arg.Secrets == nil {
			arg.Secrets = make([]corev1.ObjectReference, 1)
			arg.Secrets[0].Name = "random"
		}
	}).Once()
	acsService.On("ReconcileWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.Anything).Return(ctrl.Result{}, nil)
	clientMock.On("Get", ctx, serviceAccountSecretNamespacedName, secret).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Secret)
		if arg.Data == nil {
			arg.Data = make(map[string][]byte, 2)
		}
	})

	clientMock.On("Status").Return(clientMock)
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Update", ctx, mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil)
	ssgService.On("NodeIpReconciliationOfWorkerSliceGateways", ctx, mock.Anything, requestObj.Namespace).Return(nil)
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterUpdateSecretFail(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	// var errList errorList
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}

	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}

	secret := &corev1.Secret{}
	serviceAccountSecretNamespacedName := types.NamespacedName{
		Namespace: requestObj.Namespace,
		Name:      "random",
	}
	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil)
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.Cluster")).Return(nil).Once()
	serviceAccount := &corev1.ServiceAccount{} // Secrets: nil, //secret= nil should return requeue true and requeuetime>0
	clientMock.On("Get", ctx, mock.Anything, serviceAccount).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.ServiceAccount)
		if arg.Secrets == nil {
			arg.Secrets = make([]corev1.ObjectReference, 1)
			arg.Secrets[0].Name = "random"
		}
	}).Once()
	acsService.On("ReconcileWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.Anything).Return(ctrl.Result{}, nil)
	clientMock.On("Get", ctx, serviceAccountSecretNamespacedName, secret).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Secret)
		if arg.Data == nil {
			arg.Data = make(map[string][]byte, 2)
		}
	})
	getError := errors.New("not found")
	clientMock.On("Status").Return(clientMock)
	clientMock.On("Update", mock.Anything, mock.Anything).Return(getError)
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterServiceAccountSecretNil(t *testing.T) {
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}

	ctx := prepareTestContext(context.Background(), clientMock, nil)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	// timeStamp := kubermachine.Date(0000, 0, 0, 0, 0, 0, 0, time.UTC)
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once()
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	// finaliser
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Get", ctx, mock.AnythingOfType("types.NamespacedName"), mock.AnythingOfType("*v1alpha1.Cluster")).Return(nil).Once()
	serviceAccount := &corev1.ServiceAccount{} // Secrets: nil, //secret= nil should return requeue true and requeuetime>0
	serviceAccountNamespacedName := types.NamespacedName{
		Name:      fmt.Sprintf(ServiceAccountWorkerCluster, cluster.Name),
		Namespace: requestObj.Namespace,
	}
	clientMock.On("Get", ctx, serviceAccountNamespacedName, serviceAccount).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.ServiceAccount)
		arg.Secrets = nil
	}).Once()
	acsService.On("ReconcileWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.Anything).Return(ctrl.Result{}, nil)

	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.True(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionRaisesEvent(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	timeStamp := kubemachine.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// finaliser
	acsService.On("RemoveWorkerClusterServiceAccountAndRoleBindings", ctx, requestObj.Name, requestObj.Namespace, mock.AnythingOfType("*v1alpha1.Cluster")).Return(ctrl.Result{}, nil)
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()
	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionRequeueForDeregister(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	timeStamp := kubemachine.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
		arg.ObjectMeta.Finalizers = []string{ClusterFinalizer, ClusterDeregisterFinalizer}
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// Status Update
	clientMock.On("Status").Return(clientMock)
	clientMock.On("Update", mock.Anything, mock.Anything).Return(nil)
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.NotNil(t, result.RequeueAfter)
	require.True(t, result.RequeueAfter == 610*time.Second)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionSuccessAfterWorkerFailedToRemoveFinalizer(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	pastTimeStamp := kubemachine.NewTime(time.Now().Add(-11 * time.Minute))
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &pastTimeStamp
		arg.ObjectMeta.Finalizers = []string{ClusterFinalizer, ClusterDeregisterFinalizer}
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// Remove finalizer
	clientMock.On("Update", ctx, mock.Anything).Return(nil).Once()

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.True(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionFailureAfterWorkerFailedToRemoveFinalizer(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	pastTimeStamp := kubemachine.NewTime(time.Now().Add(-11 * time.Minute))
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &pastTimeStamp
		arg.ObjectMeta.Finalizers = []string{ClusterFinalizer, ClusterDeregisterFinalizer}
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})
	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()

	err := errors.New("failed to remove finalizer")
	clientMock.On("Update", ctx, mock.Anything).Return(err).Once()

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	_, err = clusterService.ReconcileCluster(ctx, requestObj)
	// require.True(t, result.Requeue)
	require.NotNil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionDeregisterFailed(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	timeStamp := kubemachine.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
		arg.ObjectMeta.Finalizers = []string{ClusterFinalizer, ClusterDeregisterFinalizer}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusDeregisterFailed
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// Remove finalizer

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}

func testReconcileClusterDeletionDeregisterSuccess(t *testing.T) {
	// var errList errorList
	nsServiceMock := &mocks.INamespaceService{}
	acsService := &mocks.IAccessControlService{}
	mMock := &metricMock.IMetricRecorder{}
	clusterService := ClusterService{
		ns:  nsServiceMock,
		acs: acsService,
		mf:  mMock,
	}

	clusterName := types.NamespacedName{
		Namespace: "cisco",
		Name:      "cluster-1",
	}
	requestObj := ctrl.Request{
		clusterName,
	}
	clientMock := &utilmock.Client{}
	cluster := &controllerv1alpha1.Cluster{}
	nsResource := &corev1.Namespace{}
	scheme := runtime.NewScheme()
	controllerv1alpha1.AddToScheme(scheme)
	ctx := prepareTestContext(context.Background(), clientMock, scheme)
	timeStamp := kubemachine.Now()
	mMock.On("WithProject", mock.AnythingOfType("string")).Return(&metrics.MetricRecorder{}).Once()
	clientMock.On("Get", ctx, requestObj.NamespacedName, cluster).Return(nil).Once().Run(func(args mock.Arguments) {
		arg := args.Get(2).(*controllerv1alpha1.Cluster)
		arg.ObjectMeta.DeletionTimestamp = &timeStamp
		arg.ObjectMeta.Finalizers = []string{ClusterFinalizer, ClusterDeregisterFinalizer}
		arg.Status.RegistrationStatus = controllerv1alpha1.RegistrationStatusDeregistered
	})
	clientMock.On("Get", ctx, client.ObjectKey{
		Name: requestObj.Namespace,
	}, nsResource).Return(nil).Run(func(args mock.Arguments) {
		arg := args.Get(2).(*corev1.Namespace)
		if arg.Labels == nil {
			arg.Labels = make(map[string]string)
		}
		arg.Labels[util.LabelName] = fmt.Sprintf(util.LabelValue, "Project", requestObj.Namespace)
		arg.Name = "cisco"
	})

	clientMock.On("Get", ctx, mock.Anything, mock.Anything).Return(nil).Once()
	// Remove finalizer

	clientMock.On("Create", ctx, mock.AnythingOfType("*v1.Event")).Return(nil).Once()
	mMock.On("RecordCounterMetric", mock.Anything, mock.Anything).Return().Once()
	result, err := clusterService.ReconcileCluster(ctx, requestObj)
	require.False(t, result.Requeue)
	require.Nil(t, err)
	clientMock.AssertExpectations(t)
	mMock.AssertExpectations(t)
}
