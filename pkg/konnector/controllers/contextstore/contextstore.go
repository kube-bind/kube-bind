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
	"strings"
	"sync"
)

type Key string

func (k Key) String() string {
	return string(k)
}

func NewKey(exportNamespace, exportName string, suffix ...string) Key {
	return Key(exportNamespace + "." + exportName + "." + strings.Join(suffix, "."))
}

type Store interface {
	Get(key Key) (SyncContext, bool)
	ListPrefixed(prefix Key) []SyncContext
	Set(key Key, value SyncContext)
	Delete(key Key)
	BulkDeletePrefixed(prefix Key) []SyncContext
}

type contextStore struct {
	lock  sync.Mutex
	store map[Key]SyncContext
}

type SyncContext struct {
	key        Key // reference key for the context for logging
	Generation int64
	Cancel     func()
}

func New() Store {
	return &contextStore{
		store: make(map[Key]SyncContext),
	}
}

func (c *SyncContext) Key() Key {
	return c.key
}

func (c *contextStore) Get(key Key) (SyncContext, bool) {
	c.lock.Lock()
	defer c.lock.Unlock()
	val, ok := c.store[key]
	return val, ok
}

func (c *contextStore) ListPrefixed(prefix Key) []SyncContext {
	c.lock.Lock()
	defer c.lock.Unlock()

	var results []SyncContext
	for k, v := range c.store {
		if strings.HasPrefix(k.String(), prefix.String()) {
			results = append(results, v)
		}
	}
	return results
}

func (c *contextStore) Set(key Key, value SyncContext) {
	c.lock.Lock()
	defer c.lock.Unlock()
	value.key = key
	c.store[key] = value
}

func (c *contextStore) Delete(key Key) {
	c.lock.Lock()
	defer c.lock.Unlock()
	ctx, ok := c.store[key]
	if ok {
		ctx.Cancel()
	}
	delete(c.store, key)
}

func (c *contextStore) BulkDeletePrefixed(prefix Key) []SyncContext {
	c.lock.Lock()
	defer c.lock.Unlock()

	var deleted []SyncContext
	var keysToDelete []Key

	for k, v := range c.store {
		if strings.HasPrefix(k.String(), prefix.String()) {
			keysToDelete = append(keysToDelete, k)
			deleted = append(deleted, v)
		}
	}

	for _, k := range keysToDelete {
		ctx := c.store[k]
		ctx.Cancel()
		delete(c.store, k)
	}

	return deleted
}
