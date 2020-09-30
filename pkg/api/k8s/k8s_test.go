/*
 * Tencent is pleased to support the open source community by making TKEStack available.
 *
 * Copyright (C) 2012-2019 Tencent. All Rights Reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License"); you may not use
 * this file except in compliance with the License. You may obtain a copy of the
 * License at
 *
 * https://opensource.org/licenses/Apache-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
 * WARRANTIES OF ANY KIND, either express or implied.  See the License for the
 * specific language governing permissions and limitations under the License.
 */
package k8s

import (
	"testing"
)

// #lizard forgives
func TestParsePodNetworkAnnotation(t *testing.T) {
	case1 := "test-ns/galaxy-flannel@eth0, test-ns/galaxy-k8s-vlan@eth1"
	res1, err := ParsePodNetworkAnnotation(case1)
	if err != nil {
		t.Errorf("case1 fail: %v", err)
	}
	if len(res1) == 2 {
		if res1[0].Name != "galaxy-flannel" || res1[0].InterfaceRequest != "eth0" {
			t.Errorf("network1 %s@%s not like galaxy-flannel@eth0", res1[0].Name, res1[0].InterfaceRequest)
		}
		if res1[1].Name != "galaxy-k8s-vlan" || res1[1].InterfaceRequest != "eth1" {
			t.Errorf("network2 %s@%s not like galaxy-flannel@eth1", res1[1].Name, res1[1].InterfaceRequest)
		}
	} else {
		t.Errorf("case1 network num not 2")
	}

	case3 := "test-ns/galaxy-flannel"
	res3, err := ParsePodNetworkAnnotation(case3)
	if err == nil {
		if len(res3) == 1 {
			if res3[0].Name != "galaxy-flannel" || res3[0].InterfaceRequest != "" {
				t.Errorf("case3 network isn't galaxy-flannel@{empty}")
			}
		} else {
			t.Errorf("case3 parse failed: wrong network num")
		}
	} else {
		t.Errorf("case3 parse failed")
	}
}
