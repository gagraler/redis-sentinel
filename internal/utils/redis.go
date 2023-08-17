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
	"fmt"
	"github.com/go-redis/redis"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"net"
	v1 "redis-sentinel/api/v1"
	"strconv"
	"strings"
)

// GetRedisNodesByRole Get Redis nodes by it's role i.e. master, slave and sentinel
func GetRedisNodesByRole(cr *v1.RedisReplication, redisRole string) []string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	stateFulSet, err := GetStatefulSet(cr.Namespace, cr.Name)
	if err != nil {
		logger.Error(err, "Failed to Get the stateFulSet of the", "custom resource", cr.Name, "in namespace", cr.Namespace)
	}

	var pods []string
	replicas := cr.Spec.GetReplicationCounts("replication")

	for i := 0; i < int(replicas); i++ {

		podName := stateFulSet.Name + "-" + strconv.Itoa(i)
		podRole := checkRedisServerRole(cr, podName)
		if podRole == redisRole {
			pods = append(pods, podName)
		}
	}

	return pods
}

// checkAttachedSlave would return redis pod name which has slave
func checkAttachedSlave(cr *v1.RedisReplication, masterPods []string) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	for _, podName := range masterPods {

		connectedSlaves := ""
		redisClient := configureRedisReplicationClient(cr, podName)
		defer func(redisClient *redis.Client) {
			err := redisClient.Close()
			if err != nil {
			}
		}(redisClient)
		info, err := redisClient.Info("replication").Result()
		if err != nil {
			logger.Error(err, "Failed to Get the connected slaves Info of the", "redis pod", podName)
		}

		lines := strings.Split(info, "\r\n")

		for _, line := range lines {
			if strings.HasPrefix(line, "connected_slaves:") {
				connectedSlaves = strings.TrimPrefix(line, "connected_slaves:")
				break
			}
		}

		nums, _ := strconv.Atoi(connectedSlaves)
		if nums > 0 {
			return podName
		}
	}

	return ""
}

// RedisDetails will hold the information for Redis Pod
type RedisDetails struct {
	PodName   string
	Namespace string
}

// getRedisServerIP will return the IP of redis service
func getRedisServerIP(redisInfo RedisDetails) string {
	logger := generateRedisManagerLogger(redisInfo.Namespace, redisInfo.PodName)
	redisPod, err := createKubernetesClient().CoreV1().Pods(redisInfo.Namespace).Get(context.TODO(), redisInfo.PodName, metav1.GetOptions{})
	if err != nil {
		logger.Error(err, "Error in getting redis pod IP")
	}

	redisIP := redisPod.Status.PodIP
	// If we're NOT IPv4, assume were IPv6...
	if redisIP != "" {
		if net.ParseIP(redisIP).To4() == nil {
			logger.Info("Redis is IPv6", "ip", redisIP, "ipv6", net.ParseIP(redisIP).To16())
			redisIP = fmt.Sprintf("[%s]", redisIP)
		}
	}

	logger.Info("Successfully got the ip for redis", "ip", redisIP)
	return redisIP
}

// Check the Redis Server Role i.e. master, slave and sentinel
func checkRedisServerRole(cr *v1.RedisReplication, podName string) string {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)

	redisClient := configureRedisReplicationClient(cr, podName)
	defer func(redisClient *redis.Client) {
		err := redisClient.Close()
		if err != nil {
		}
	}(redisClient)
	info, err := redisClient.Info("replication").Result()
	if err != nil {
		logger.Error(err, "Failed to Get the role Info of the", "redis pod", podName)
	}

	lines := strings.Split(info, "\r\n")
	role := ""
	for _, line := range lines {
		if strings.HasPrefix(line, "role:") {
			role = strings.TrimPrefix(line, "role:")
			break
		}
	}

	return role
}

// configureRedisClient will configure the Redis Client
func configureRedisReplicationClient(cr *v1.RedisReplication, podName string) *redis.Client {
	logger := generateRedisManagerLogger(cr.Namespace, cr.ObjectMeta.Name)
	redisInfo := RedisDetails{
		PodName:   podName,
		Namespace: cr.Namespace,
	}
	var client *redis.Client

	if cr.Spec.RedisConfig.ExistingPasswordSecret != nil {
		pass, err := getRedisPassword(cr.Namespace, *cr.Spec.RedisConfig.ExistingPasswordSecret.Name, *cr.Spec.RedisConfig.ExistingPasswordSecret.Key)
		if err != nil {
			logger.Error(err, "Error in getting redis password")
		}
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  pass,
			DB:        0,
			TLSConfig: getRedisReplicationTLSConfig(cr, redisInfo),
		})
	} else {
		client = redis.NewClient(&redis.Options{
			Addr:      getRedisServerIP(redisInfo) + ":6379",
			Password:  "",
			DB:        0,
			TLSConfig: getRedisReplicationTLSConfig(cr, redisInfo),
		})
	}
	return client
}
