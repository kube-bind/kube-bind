/*
Copyright 2023 The Kube Bind Authors.

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

package plugin

import (
	"bytes"
	"os"
	"testing"

	"k8s.io/cli-runtime/pkg/genericclioptions"

	kubebindv1alpha1 "github.com/kube-bind/kube-bind/pkg/apis/kubebind/v1alpha1"
)

func TestHumanReadablePromt(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		testData       kubebindv1alpha1.PermissionClaim
		expectedOutput string
	}{
		{"Owner=Provider",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Required=false",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: false,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Selector.Name",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name:  "bar",
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider and to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,GroupResource.Group",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "example.com",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Selector.Name,GroupResource.Group",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "example.com",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name:  "bar",
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") on your cluster. This only applies to objects which are owned by the provider and to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,CreateOptions={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Create:   &kubebindv1alpha1.CreateOptions{},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,CreateOptions=false",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: false,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,CreateOption.Donate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: true,
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required:   true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.ProviderOverwrites=false",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					ProviderOverwrites: false,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.ProviderOverwrites=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					ProviderOverwrites: true,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Conflicting objects will be overwritten and created objects will not be recreated upon deletion.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.RecreateWhenConsumerSideDeleted=false",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					RecreateWhenConsumerSideDeleted: false,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.RecreateWhenConsumerSideDeleted=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					RecreateWhenConsumerSideDeleted: true,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Conflicting objects will not be overwritten and created objects will be recreated upon deletion.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Update:   &kubebindv1alpha1.UpdateOptions{},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Fields",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"foo", "bar"},
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Preserving",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Preserving: []string{"foo", "bar"},
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"The following fields of the objects will be overwritten with their initial values, if they are modified:\n" + "	\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.AlwaysRecreate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					AlwaysRecreate: true,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"Modification of said objects will by handled by deletion and recreation of said objects.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Fields,CreateOptions.Donate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: true,
				},
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"foo", "bar"},
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"The following fields of the objects will still be able to be changed by the user:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Preserving,CreateOptions.Donate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Provider,
				},
				Required: true,
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: true,
				},
				Update: &kubebindv1alpha1.UpdateOptions{
					Preserving: []string{"foo", "bar"},
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by the provider.\n" +
				"The following fields of the objects will be overwritten with their initial values, if they are modified:\n" + "	\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Name",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name:  "bar",
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you and to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,GroupResource.Group",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "example.com",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Name,GroupResource.Group",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "example.com",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name:  "bar",
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") on your cluster. This only applies to objects which are owned by you and to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Adopt=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Adopt:    true,
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Name,Adopt=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name:  "bar",
					Owner: kubebindv1alpha1.Consumer,
				},
				Adopt:    true,
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you and to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,OnConflict={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required:   true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,OnConflict.ProviderOverwrites=false",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					ProviderOverwrites: false,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,OnConflict.ProviderOverwrites=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				OnConflict: &kubebindv1alpha1.OnConflictOptions{
					ProviderOverwrites: true,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Conflicting objects will be overwritten and created objects will not be recreated upon deletion.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Update:   &kubebindv1alpha1.UpdateOptions{},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Fields",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"foo", "bar"},
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Preserving",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Preserving: []string{"foo", "bar"},
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"The following fields of the objects will be overwritten with their initial values, if they are modified:\n" + "	\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.AlwaysRecreate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Update: &kubebindv1alpha1.UpdateOptions{
					AlwaysRecreate: true,
				},
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"Modification of said objects will by handled by deletion and recreation of said objects.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Fields,Adopt=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Adopt:    true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"foo", "bar"},
				},
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Preserving,Adopt=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Owner: kubebindv1alpha1.Consumer,
				},
				Required: true,
				Adopt:    true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Preserving: []string{"foo", "bar"},
				},
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are owned by you.\n" +
				"The following fields of the objects will be overwritten with their initial values, if they are modified:\n" + "	\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={}",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version:  "v1",
				Selector: kubebindv1alpha1.ResourceSelector{},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector.Owner=\"\",Selector.Name",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version: "v1",
				Selector: kubebindv1alpha1.ResourceSelector{
					Name: "bar",
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster. This only applies to objects which are referenced with:\n" +
				"\tname: \"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},donate=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version:  "v1",
				Selector: kubebindv1alpha1.ResourceSelector{},
				Required: true,
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: true,
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},donate=true,update.Fields=[\"spec\"]",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version:  "v1",
				Selector: kubebindv1alpha1.ResourceSelector{},
				Create: &kubebindv1alpha1.CreateOptions{
					Donate: true,
				},
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"spec"},
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the user:\n" +
				"\t\"spec\"\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},adopt=true",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version:  "v1",
				Selector: kubebindv1alpha1.ResourceSelector{},
				Required: true,
				Adopt:    true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},adopt=true,update.Fields=[\"spec\"]",
			kubebindv1alpha1.PermissionClaim{
				GroupResource: kubebindv1alpha1.GroupResource{
					Group:    "",
					Resource: "foo",
				},
				Version:  "v1",
				Selector: kubebindv1alpha1.ResourceSelector{},
				Adopt:    true,
				Update: &kubebindv1alpha1.UpdateOptions{
					Fields: []string{"spec"},
				},
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"spec\"\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var output bytes.Buffer
			var input bytes.Buffer
			input.WriteString("y\n")
			opts := NewBindAPIServiceOptions(genericclioptions.IOStreams{In: &input, Out: &output, ErrOut: os.Stderr})
			b, err := opts.promptYesNo(tt.testData)
			if output.String() != tt.expectedOutput {
				t.Errorf("Expected IO Output did not match. got: \"\n%s\"\nwanted: \"\n%s\"\n", output.String(), tt.expectedOutput)
			}
			if b == false || (err != nil) {
				t.Errorf("Expected Return value did not match. got: \"%v\", \"%v\"", b, err)
			}
		})
	}
}
