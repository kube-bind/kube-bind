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
		testData       kubebindv1alpha1.ExportPermissionClaim
		expectedOutput string
	}{
		{"Owner=Provider",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Required=false",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: false,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Selector.Names={foo}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,GroupResource.Group",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "example.com",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"example.com/v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Selector.Names={bar},GroupResource.Group",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "example.com",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"example.com/v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,CreateOptions={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Create: &kubebindv1alpha1.PermissionClaimCreateOptions{},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,AutoDonate=false",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferDonate,
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,AutoDonate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferDonate,
				},
				Required: true,
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					OnConflict: &kubebindv1alpha1.PermissionClaimOnConflictOptions{},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Create.ReplaceExisting=false",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Create: &kubebindv1alpha1.PermissionClaimCreateOptions{
						ReplaceExisting: false,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Create.ReplaceExisting=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Create: &kubebindv1alpha1.PermissionClaimCreateOptions{
						ReplaceExisting: true,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Conflicting objects will be replaced by the provider. " +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.RecreateWhenConsumerSideDeleted=false",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					OnConflict: &kubebindv1alpha1.PermissionClaimOnConflictOptions{
						RecreateWhenConsumerSideDeleted: false,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,OnConflict.RecreateWhenConsumerSideDeleted=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					OnConflict: &kubebindv1alpha1.PermissionClaimOnConflictOptions{
						RecreateWhenConsumerSideDeleted: true,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Created objects will be recreated upon deletion. " +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Fields",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"foo", "bar"},
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" + // TODO
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Preserving",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Preserving: []string{"foo", "bar"},
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will be preserved by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.AlwaysRecreate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						AlwaysRecreate: true,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Modification of said objects will by handled by deletion and recreation of said objects.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Fields,AutoDonate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"foo", "bar"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferDonate,
				},
				Required: true,
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,UpdateOptions.Preserving,AutoDonate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Provider,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Preserving: []string{"foo", "bar"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferDonate,
				},
				Required: true,
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will be preserved by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Names={bar}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
						Owner: kubebindv1alpha1.Consumer,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,GroupResource.Group",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "example.com",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Names={bar},GroupResource.Group",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "example.com",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
						Owner: kubebindv1alpha1.Consumer,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"example.com/v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Adopt=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferAdopt,
				},
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Selector.Names={bar},Adopt=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
						Owner: kubebindv1alpha1.Consumer,
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferAdopt,
				},
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,OnConflict={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					OnConflict: &kubebindv1alpha1.PermissionClaimOnConflictOptions{},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Create.ReplaceExisting=false",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Create: &kubebindv1alpha1.PermissionClaimCreateOptions{
						ReplaceExisting: false,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,Create.ReplaceExisting=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Create: &kubebindv1alpha1.PermissionClaimCreateOptions{
						ReplaceExisting: true,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Conflicting objects will be replaced by the provider. " +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Fields",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"foo", "bar"},
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Preserving",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Preserving: []string{"foo", "bar"},
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will be preserved by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.AlwaysRecreate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						AlwaysRecreate: true,
					},
				},
				Required: true,
			},
			"The provider wants to read foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Modification of said objects will by handled by deletion and recreation of said objects.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Fields,Adopt=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"foo", "bar"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferAdopt,
				},
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Consumer,UpdateOptions.Preserving,Adopt=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Owner: kubebindv1alpha1.Consumer,
					},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Preserving: []string{"foo", "bar"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferAdopt,
				},
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will be preserved by the provider:\n" + "	\"foo\"\n" +
				"\t\"bar\"\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version:        "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{},
				},
				Required: true,
			},
			"The provider wants to read and write foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector.Owner=\"\",Selector.Names={bar}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar"},
					},
				},
				Required: true,
			},
			"The provider wants to read and write foo objects (apiVersion: \"v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},AutoDonate=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version:        "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{},
					OwnerTransfer:  kubebindv1alpha1.OwnerTransferDonate,
				},
				Required: true,
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},AutoDonate=true,update.Fields=[\"spec\"]",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version:        "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"spec"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferDonate,
				},
			},
			"The provider wants to create user owned foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"spec\"\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},adopt=true",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version:        "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{},
					OwnerTransfer:  kubebindv1alpha1.OwnerTransferAdopt,
				},
				Required: true,
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Selector={},adopt=true,update.Fields=[\"spec\"]",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version:        "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{},
					Update: &kubebindv1alpha1.PermissionClaimUpdateOptions{
						Fields: []string{"spec"},
					},
					OwnerTransfer: kubebindv1alpha1.OwnerTransferAdopt,
				},
			},
			"The provider wants to have ownership of foo objects (apiVersion: \"v1\") on your cluster.\n" +
				"The following fields of the objects will still be able to be changed by the provider:\n" +
				"\t\"spec\"\n" +
				"Accepting this Permission is optional.\n" +
				"Do you accept this Permission? [No,Yes]\n",
		},
		{"Owner=Provider,Selector.Names={bar,baz}",
			kubebindv1alpha1.ExportPermissionClaim{
				PermissionClaim: kubebindv1alpha1.PermissionClaim{
					GroupResource: kubebindv1alpha1.GroupResource{
						Group:    "",
						Resource: "foo",
					},
					Version: "v1",
					ObjectSelector: &kubebindv1alpha1.ObjectSelector{
						Names: []string{"bar", "baz"},
						Owner: kubebindv1alpha1.Provider,
					},
				},
				Required: true,
			},
			"The provider wants to write foo objects (apiVersion: \"v1\") which are referenced with:\n" +
				"\t- name: \"bar\"\n" +
				"\t- name: \"baz\"\n" +
				"on your cluster.\n" +
				"Accepting this Permission is required in order to proceed.\n" +
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
