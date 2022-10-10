/*
Copyright 2022 The kube bind Authors.

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

package dynamic

import (
	"context"
	"sync"

	"k8s.io/client-go/tools/cache"
)

type SharedIndexInformer interface {
	// AddDynamicEventHandler adds a dynamic event handler to the informer. It's
	// like AddEventHandler, but the handler is removed when the context closes.
	AddDynamicEventHandler(ctx context.Context, handler cache.ResourceEventHandler)

	// AddEventHandler shadows the method in the embedded SharedIndexInformer. But it
	// will panic and should not be called.
	AddEventHandler(handler cache.ResourceEventHandler)

	cache.SharedIndexInformer
}

type Informer[L any] interface {
	Informer() SharedIndexInformer
	Lister() L
}

type StaticInformer[L any] interface {
	Informer() cache.SharedIndexInformer
	Lister() L
}

type dynamicInformer[L any] struct {
	StaticInformer[L]
	sharedIndexInformer dynamicSharedIndexInformer
}

type dynamicSharedIndexInformer struct {
	cache.SharedIndexInformer

	lock     sync.RWMutex
	handlers map[cache.ResourceEventHandler]struct{}
}

// NewDynamicInformer returns a shared informer that allows adding and removing event
// handlers dynamically.
func NewDynamicInformer[L any](informer StaticInformer[L]) Informer[L] {
	di := &dynamicInformer[L]{
		StaticInformer: informer,
		sharedIndexInformer: dynamicSharedIndexInformer{
			SharedIndexInformer: informer.Informer(),
			handlers:            make(map[cache.ResourceEventHandler]struct{}),
		},
	}

	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			di.sharedIndexInformer.lock.RLock()
			defer di.sharedIndexInformer.lock.RLock()
			for h := range di.sharedIndexInformer.handlers {
				h.OnAdd(obj)
			}
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			di.sharedIndexInformer.lock.RLock()
			defer di.sharedIndexInformer.lock.RLock()
			for h := range di.sharedIndexInformer.handlers {
				h.OnUpdate(oldObj, newObj)
			}
		},
		DeleteFunc: func(obj interface{}) {
			di.sharedIndexInformer.lock.RLock()
			defer di.sharedIndexInformer.lock.RLock()
			for h := range di.sharedIndexInformer.handlers {
				h.OnDelete(obj)
			}
		},
	})

	return di
}

func (i *dynamicInformer[L]) Informer() SharedIndexInformer {
	return &i.sharedIndexInformer
}

func (i *dynamicInformer[L]) Lister() L {
	return i.StaticInformer.Lister()
}

func (i *dynamicSharedIndexInformer) AddDynamicEventHandler(ctx context.Context, handler cache.ResourceEventHandler) {
	i.lock.Lock()
	defer i.lock.Unlock()
	i.handlers[handler] = struct{}{}

	go func() {
		<-ctx.Done()
		i.lock.Lock()
		defer i.lock.Unlock()
		delete(i.handlers, handler)
	}()
}

func (i *dynamicSharedIndexInformer) AddEventHandler(handler cache.ResourceEventHandler) {
	panic("call AddDynamicEventHandler instead")
}
