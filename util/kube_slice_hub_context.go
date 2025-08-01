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

package util

import (
	"context"
	"github.com/kubeslice/kubeslice-monitoring/pkg/events"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime"
	k8sUuid "k8s.io/apimachinery/pkg/util/uuid"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var Loglevel zapcore.Level
var LoglevelString string

type kubeSliceControllerContextKey struct {
}

// kubeSliceControllerRequestContext is a schema for request context
type kubeSliceControllerRequestContext struct {
	client.Client
	Scheme        *runtime.Scheme
	Log           *zap.SugaredLogger
	eventRecorder *events.EventRecorder
}

// kubeSliceControllerContext is instance of kubeSliceControllerContextKey
var kubeSliceControllerContext = &kubeSliceControllerContextKey{}

// PrepareKubeSliceControllersRequestContext is a function to create the context for kube slice
func PrepareKubeSliceControllersRequestContext(ctx context.Context, client client.Client,
	scheme *runtime.Scheme, controllerName string, er *events.EventRecorder) context.Context {
	uuid := k8sUuid.NewUUID()[:8]

	var log *zap.SugaredLogger

	if Loglevel == zap.DebugLevel {
		log = NewLogger().With(
			zap.String("RequestId", string(uuid)),
			zap.String("Controller", controllerName),
		)
	} else {
		log = NewLogger()
	}

	ctxVal := &kubeSliceControllerRequestContext{
		Client:        client,
		Scheme:        scheme,
		Log:           log,
		eventRecorder: er,
	}
	newCtx := context.WithValue(ctx, kubeSliceControllerContext, ctxVal)
	return newCtx
}

// GetKubeSliceControllerRequestContext is a function to get the request context
func GetKubeSliceControllerRequestContext(ctx context.Context) *kubeSliceControllerRequestContext {
	if ctx.Value(kubeSliceControllerContext) != nil {
		return ctx.Value(kubeSliceControllerContext).(*kubeSliceControllerRequestContext)
	}
	return nil
}

// CtxLogger is a function to get the logs
func CtxLogger(ctx context.Context) *zap.SugaredLogger {
	logg := GetKubeSliceControllerRequestContext(ctx).Log
	return logg
}

// CtxClient is a function to get the Client
func CtxClient(ctx context.Context) client.Client {
	return GetKubeSliceControllerRequestContext(ctx).Client
}

// CtxScheme is a function to get the Scheme
func CtxScheme(ctx context.Context) *runtime.Scheme {
	return GetKubeSliceControllerRequestContext(ctx).Scheme
}

// CtxEventRecorder is a function to get the EventRecorder
func CtxEventRecorder(ctx context.Context) events.EventRecorder {
	recorder := GetKubeSliceControllerRequestContext(ctx).eventRecorder
	return *recorder
}
