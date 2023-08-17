/*
 *
 * Copyright 2023 keington.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 * /
 */

package utils

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	v1 "redis-sentinel/api/v1"
	"strings"
)

// getRedisPassword method will return the redis password
func getRedisPassword(namespace, name, secretKey string) (string, error) {
	logger := secretLogger(namespace, name)
	secretName, err := createKubernetesClient().CoreV1().Secrets(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Failed in getting existing secret for redis")
		return "", err
	}
	for key, value := range secretName.Data {
		if key == secretKey {
			return strings.TrimSpace(string(value)), nil
		}
	}
	return "", nil
}

func secretLogger(namespace string, name string) logr.Logger {
	reqLogger := log.WithValues("Request.Secret.Namespace", namespace, "Request.Secret.Name", name)
	return reqLogger
}

func getRedisReplicationTLSConfig(cr *v1.RedisReplication, redisInfo RedisDetails) *tls.Config {
	if cr.Spec.TLS != nil {
		reqLogger := log.WithValues("Request.Namespace", cr.Namespace, "Request.Name", cr.ObjectMeta.Name)
		secretName, err := createKubernetesClient().CoreV1().Secrets(cr.Namespace).Get(context.TODO(), cr.Spec.TLS.Secret.SecretName, metav1.GetOptions{})
		if err != nil {
			reqLogger.Error(err, "Failed in getting TLS secret for redis")
		}

		var (
			tlsClientCert         []byte
			tlsClientKey          []byte
			tlsCaCertificate      []byte
			tlsCaCertificates     *x509.CertPool
			tlsClientCertificates []tls.Certificate
		)
		for key, value := range secretName.Data {
			if key == cr.Spec.TLS.CaKeyFile || key == "ca.crt" {
				tlsCaCertificate = value
			} else if key == cr.Spec.TLS.CertKeyFile || key == "tls.crt" {
				tlsClientCert = value
			} else if key == cr.Spec.TLS.KeyFile || key == "tls.key" {
				tlsClientKey = value
			}
		}

		cert, err := tls.X509KeyPair(tlsClientCert, tlsClientKey)
		if err != nil {
			reqLogger.Error(err, "Couldn't load TLS client key pair")
		}
		tlsClientCertificates = append(tlsClientCertificates, cert)

		tlsCaCertificates = x509.NewCertPool()
		ok := tlsCaCertificates.AppendCertsFromPEM(tlsCaCertificate)
		if !ok {
			reqLogger.Info("Failed to load CA Certificates from Secret")
		}

		return &tls.Config{
			Certificates: tlsClientCertificates,
			ServerName:   redisInfo.PodName,
			RootCAs:      tlsCaCertificates,
			MinVersion:   2,
			ClientAuth:   0,
		}
	}
	return nil
}
