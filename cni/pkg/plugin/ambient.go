// Copyright Istio Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package plugin

import (
	"context"
	"fmt"
	"net"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"istio.io/istio/cni/pkg/ambient"
	"istio.io/istio/cni/pkg/ambient/ambientpod"
)

func checkAmbient(conf Config, ambientConfig ambient.AmbientConfigFile, podName, podNamespace, podIfname string, podIPs []net.IPNet) (bool, error) {
	if !ambientConfig.ZTunnelReady {
		return false, fmt.Errorf("ztunnel not ready")
	}

	client, err := newKubeClient(conf)
	if err != nil {
		return false, err
	}

	if client == nil {
		return false, nil
	}

	pod, err := client.CoreV1().Pods(podNamespace).Get(context.Background(), podName, metav1.GetOptions{})
	if err != nil {
		return false, err
	}
	ns, err := client.CoreV1().Namespaces().Get(context.Background(), podNamespace, metav1.GetOptions{})
	if err != nil {
		return false, err
	}

	if ambientpod.HasLegacyLabel(pod.Labels) || ambientpod.HasLegacyLabel(ns.Labels) {
		return false, fmt.Errorf("ambient: pod %s/%s or namespace has legacy labels", podNamespace, podName)
	}

	if ambientpod.ShouldPodBeInIpset(ns, pod, true) {
		ambient.NodeName = pod.Spec.NodeName

		ambient.HostIP, err = ambient.GetHostIP(client)
		if err != nil || ambient.HostIP == "" {
			return false, fmt.Errorf("error getting host IP: %v", err)
		}

		// Can't set this on GKE, but needed in AWS.. so silently ignore failures
		_ = ambient.SetProc("/proc/sys/net/ipv4/conf/"+podIfname+"/rp_filter", "0")

		for _, ip := range podIPs {
			ambient.AddPodToMesh(client, pod, ip.IP.String())
		}
		return true, nil
	}

	return false, nil
}
