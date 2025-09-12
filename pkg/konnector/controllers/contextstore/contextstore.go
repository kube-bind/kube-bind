/*
Copyright 2022 The Kube Bind Authors.

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

package contextstore

// contextstore allows to manage and track context per controllers, stored at the different levels of the controller hierarchy:
// APIServiceExport - schemas and permissionClaims - each schema runs its own gvr controller and each permissionClaim runs its own controller.
// Context are stored at the APIServiceExport level.

import (
	"sync"

	kubebindv1alpha2 "github.com/kube-bind/kube-bind/sdk/apis/kubebind/v1alpha2"
)

type Key string

func (k Key) String() string {
	return string(k)
}

func NewKey(export kubebindv1alpha2.APIServiceExport) Key {
	return Key(export.Namespace + "/" + export.Name)
}

type ContextStore interface {
	Get(key Key) (SyncContext, bool)
	Set(key Key, value SyncContext)
	Delete(key Key)
}

type contextStore struct {
	lock  sync.Mutex
	store map[Key]SyncContext
}

type SyncContext struct {
	generation int64
	cancel     func()
}

func New() ContextStore {
	return &contextStore{
		store: make(map[Key]SyncContext),
	}
}

func (c *SyncContext) Generation() int64 {
	return c.generation
}

func (c *contextStore) Get(key Key) (SyncContext, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	val, ok := c.store[key]
	return val, ok
}

func (c *contextStore) Set(key Key, value SyncContext) {
	c.lock.Lock()
	defer c.lock.Unlock()
	c.store[key] = value
}

func (c *contextStore) Delete(key Key) {
	c.lock.Lock()
	defer c.lock.Unlock()
	ctx, ok := c.store[key]
	if ok {
		ctx.cancel()
	}
	delete(c.store, key)
}
