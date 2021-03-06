package cache

import (
	"fmt"

	"github.com/ttsubo/client-go/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	v1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
)

// Store is a generic object storage and processing interface.
type Store interface {

	// Add adds the given object to the accumulator associated with the given object's key
	Add(obj interface{}) error

	// Update updates the given object in the accumulator associated with the given object's key
	Update(obj interface{}) error

	// Delete deletes the given object from the accumulator associated with the given object's key
	Delete(obj interface{}) error

	// List returns a list of all the currently non-empty accumulators
	List() []interface{}

	// Get returns the accumulator associated with the given object's key
	Get(obj interface{}) (item interface{}, exists bool, err error)

	// GetByKey returns the accumulator associated with the given key
	GetByKey(key string) (item interface{}, exists bool, err error)

	AddInformer(obj runtime.Object, informer cache.Controller)
}

// KeyFunc knows how to make a key from an object. Implementations should be deterministic.
type KeyFunc func(obj interface{}) (string, error)

// KeyError will be returned any time a KeyFunc gives an error; it includes the object
// at fault.
type KeyError struct {
	Obj interface{}
	Err error
}

// Error gives a human-readable description of the error.
func (k KeyError) Error() string {
	return fmt.Sprintf("couldn't create key for object %+v: %v", k.Obj, k.Err)
}

// Unwrap implements errors.Unwrap
func (k KeyError) Unwrap() error {
	return k.Err
}

// MetaNamespaceKeyFunc is a convenient default KeyFunc which knows how to make
// keys for API objects which implement meta.Interface.
func MetaNamespaceKeyFunc(obj interface{}) (string, error) {
	meta, err := meta.Accessor(obj)
	if err != nil {
		return "", fmt.Errorf("object has no meta: %v", err)
	}
	if len(meta.GetNamespace()) > 0 && len(meta.GetGenerateName()) > 0 {
		return meta.GetGenerateName() + "/" + meta.GetNamespace() + "/" + meta.GetName(), nil
	}
	return meta.GetName(), nil
}

// `*cache` implements Indexer in terms of a ThreadSafeStore and an
// associated KeyFunc.
type Cache struct {
	// cacheStorage bears the burden of thread safety for the cache
	cacheStorage ThreadSafeStore
	// keyFunc is used to make the key for objects stored in and retrieved from items, and
	// should be deterministic.
	keyFunc KeyFunc

	// setup informer
	InformerConfig   cache.Controller
	InformerProvider cache.Controller
	InformerSecret   cache.Controller
	InformerJob      cache.Controller
}

//var _ Store = &cache{}

// Add inserts an item into the cache.
func (c *Cache) Add(obj interface{}) error {
	key, err := c.keyFunc(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	c.cacheStorage.Add(key, obj)

	switch obj.(type) {
	case *types.Provider:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Provider))
		c.InformerProvider.InjectWorkerQueue(obj)
	case *types.Secret:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Secret))
		c.InformerSecret.InjectWorkerQueue(obj)
	case *types.Configuration:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Configuration))
		c.InformerConfig.InjectWorkerQueue(obj)
	case *types.Job:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.Job))
		c.InformerJob.InjectWorkerQueue(obj)
	case *types.ConfigMap:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*types.ConfigMap))
	case *rbacv1.ClusterRole:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*rbacv1.ClusterRole))
	case *v1.ServiceAccount:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*v1.ServiceAccount))
	case *rbacv1.ClusterRoleBinding:
		klog.Infof("Update key:[%s], obj:[%v]", key, obj.(*rbacv1.ClusterRoleBinding))
	}
	return nil
}

// Update sets an item in the cache to its updated state.
func (c *Cache) Update(obj interface{}) error {
	key, err := c.keyFunc(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	c.cacheStorage.Update(key, obj)
	return nil
}

// Delete removes an item from the cache.
func (c *Cache) Delete(obj interface{}) error {
	key, err := c.keyFunc(obj)
	if err != nil {
		return KeyError{obj, err}
	}
	c.cacheStorage.Delete(key)
	return nil
}

// List returns a list of all the items.
// List is completely threadsafe as long as you treat all items as immutable.
func (c *Cache) List() []interface{} {
	return c.cacheStorage.List()
}

// Get returns the requested item, or sets exists=false.
// Get is completely threadsafe as long as you treat all items as immutable.
func (c *Cache) Get(obj interface{}) (item interface{}, exists bool, err error) {
	key, err := c.keyFunc(obj)
	if err != nil {
		return nil, false, KeyError{obj, err}
	}
	return c.GetByKey(key)
}

// GetByKey returns the request item, or exists=false.
// GetByKey is completely threadsafe as long as you treat all items as immutable.
func (c *Cache) GetByKey(key string) (item interface{}, exists bool, err error) {
	item, exists = c.cacheStorage.Get(key)
	if exists == false {
		return item, exists, fmt.Errorf("cannot find obj from store... ")
	} else {
		return item, exists, nil
	}
}

// Add Informer
func (c *Cache) AddInformer(obj runtime.Object, informer cache.Controller) {
	switch obj.(type) {
	case *types.Secret:
		c.InformerSecret = informer
	case *types.Provider:
		c.InformerProvider = informer
	case *types.Configuration:
		c.InformerConfig = informer
	case *types.Job:
		c.InformerJob = informer
	}
}

// NewStore returns a Store implemented simply with a map and a lock.
func NewStore(keyFunc KeyFunc) Store {
	return &Cache{
		cacheStorage: NewThreadSafeStore(),
		keyFunc:      keyFunc,
	}
}
