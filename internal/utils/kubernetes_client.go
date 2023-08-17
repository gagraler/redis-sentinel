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
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// createKubernetesClient 创建kubernetes客户端
func createKubernetesClient() *kubernetes.Clientset {
	config, err := loadKubeConfig()
	if err != nil {
		panic(err.Error())
	}
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return clientSet
}

// createK8sDynamicClient create Dynamic client for kubernetes
func createK8sDynamicClient() dynamic.Interface {
	config, err := createK8sConfig()
	if err != nil {
		panic(err.Error())
	}
	dynamicClientSet, err := dynamic.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	return dynamicClientSet
}

// generateK8sConfig will load the kube config file
func createK8sConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// if you want to change the loading rules (which files in which order), you can do so here
	configOverrides := &clientcmd.ConfigOverrides{}
	// if you want to change override values or bind them to flags, there are methods to help you
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig()
}

// loadKubeConfig 加载 kubeConfig 文件
func loadKubeConfig() (*rest.Config, error) {
	loadingRules := clientcmd.NewDefaultClientConfigLoadingRules()
	// 自定义kubeConfig文件路径
	configOverrides := &clientcmd.ConfigOverrides{}
	// 绑定到标志
	kubeConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, configOverrides)
	return kubeConfig.ClientConfig()
}
