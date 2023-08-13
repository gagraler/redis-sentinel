/*
Copyright 2023 keington.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package utils

import (
	"context"
	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	redisSentinelv1 "redis-sentinel/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strconv"
)

var (
	log logr.Logger
)

const (
	redisSentinelFinalizer string = "RedisSentinelFinalizer"
)

// finalizerLogger 终结器接口的记录器
func finalizerLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Service.Namespace", namespace, "Request.Finalizer.Name", name)
	return reqLogger
}

// HandleRedisSentinelFinalizer 处理终结器
// 如果实例被标记为删除，则完成资源及其清理工作
func HandleRedisSentinelFinalizer(cr *redisSentinelv1.RedisSentinel, cli client.Client) error {

	logger := finalizerLogger(cr.Namespace, redisSentinelFinalizer)

	// 如果对象被删除
	if cr.GetDeletionTimestamp() != nil {
		// 如果终结器不存在
		if controllerutil.ContainsFinalizer(cr, redisSentinelFinalizer) {
			if err := finalizeRedisSentinelPVC(cr); err != nil {
				return err
			}
			// 删除终结器
			controllerutil.RemoveFinalizer(cr, redisSentinelFinalizer)
			if err := cli.Update(context.TODO(), cr); err != nil {
				logger.Error(err, "Failed to update RedisSentinel with finalizer"+redisSentinelFinalizer)
				return err
			}
		}
	}

	return nil
}

// AddRedisSentinelFinalizer 添加终结器
func AddRedisSentinelFinalizer(cr *redisSentinelv1.RedisSentinel, cl client.Client) error {
	if !controllerutil.ContainsFinalizer(cr, redisSentinelFinalizer) {
		controllerutil.AddFinalizer(cr, redisSentinelFinalizer)
		return cl.Update(context.TODO(), cr)
	}
	return nil
}

// finalizeRedisSentinelPVC 清理 PVC
func finalizeRedisSentinelPVC(cr *redisSentinelv1.RedisSentinel) error {
	logger := finalizerLogger(cr.Namespace, redisSentinelFinalizer)

	for i := 0; i < int(cr.Spec.GetSentinelCounts("SentinelCounts")); i++ {
		pvcName := cr.Name + "-" + cr.Name + "-" + strconv.Itoa(i)
		err := createKubernetesClient().CoreV1().PersistentVolumeClaims(cr.Name).Delete(context.TODO(), pvcName, metav1.DeleteOptions{})
		if err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "Could not delete Persistent Volume Claim "+pvcName)
			return err
		}
	}

	return nil
}
