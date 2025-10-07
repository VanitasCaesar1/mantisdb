// Package container provides dependency injection functionality
package container

import (
	"fmt"
	"reflect"
	"sync"
)

// Container manages dependency injection
type Container struct {
	services   map[string]interface{}
	factories  map[string]func() interface{}
	singletons map[string]interface{}
	mu         sync.RWMutex
}

// NewContainer creates a new dependency injection container
func NewContainer() *Container {
	return &Container{
		services:   make(map[string]interface{}),
		factories:  make(map[string]func() interface{}),
		singletons: make(map[string]interface{}),
	}
}

// Register registers a service instance
func (c *Container) Register(name string, service interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.services[name] = service
}

// RegisterFactory registers a factory function for creating services
func (c *Container) RegisterFactory(name string, factory func() interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factories[name] = factory
}

// RegisterSingleton registers a singleton service that will be created once
func (c *Container) RegisterSingleton(name string, factory func() interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.factories[name] = factory
}

// Get retrieves a service by name
func (c *Container) Get(name string) (interface{}, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()

	// Check for direct service registration
	if service, exists := c.services[name]; exists {
		return service, nil
	}

	// Check for singleton
	if singleton, exists := c.singletons[name]; exists {
		return singleton, nil
	}

	// Check for factory
	if factory, exists := c.factories[name]; exists {
		service := factory()

		// Store as singleton if it was registered as one
		c.singletons[name] = service
		return service, nil
	}

	return nil, fmt.Errorf("service '%s' not found", name)
}

// MustGet retrieves a service by name and panics if not found
func (c *Container) MustGet(name string) interface{} {
	service, err := c.Get(name)
	if err != nil {
		panic(err)
	}
	return service
}

// GetTyped retrieves a service by name and casts it to the specified type
func (c *Container) GetTyped(name string, target interface{}) error {
	service, err := c.Get(name)
	if err != nil {
		return err
	}

	serviceValue := reflect.ValueOf(service)
	targetValue := reflect.ValueOf(target)

	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetType := targetValue.Elem().Type()
	if !serviceValue.Type().AssignableTo(targetType) {
		return fmt.Errorf("service type %s is not assignable to target type %s",
			serviceValue.Type(), targetType)
	}

	targetValue.Elem().Set(serviceValue)
	return nil
}

// Inject performs dependency injection on a struct
func (c *Container) Inject(target interface{}) error {
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}

	targetValue = targetValue.Elem()
	if targetValue.Kind() != reflect.Struct {
		return fmt.Errorf("target must be a pointer to struct")
	}

	targetType := targetValue.Type()

	for i := 0; i < targetValue.NumField(); i++ {
		field := targetValue.Field(i)
		fieldType := targetType.Field(i)

		// Check for inject tag
		injectTag := fieldType.Tag.Get("inject")
		if injectTag == "" {
			continue
		}

		// Skip if field is not settable
		if !field.CanSet() {
			continue
		}

		// Get service from container
		service, err := c.Get(injectTag)
		if err != nil {
			return fmt.Errorf("failed to inject field %s: %w", fieldType.Name, err)
		}

		serviceValue := reflect.ValueOf(service)
		if !serviceValue.Type().AssignableTo(field.Type()) {
			return fmt.Errorf("service type %s is not assignable to field type %s",
				serviceValue.Type(), field.Type())
		}

		field.Set(serviceValue)
	}

	return nil
}

// Has checks if a service is registered
func (c *Container) Has(name string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()

	_, hasService := c.services[name]
	_, hasFactory := c.factories[name]
	_, hasSingleton := c.singletons[name]

	return hasService || hasFactory || hasSingleton
}

// Remove removes a service from the container
func (c *Container) Remove(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	delete(c.services, name)
	delete(c.factories, name)
	delete(c.singletons, name)
}

// Clear removes all services from the container
func (c *Container) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.services = make(map[string]interface{})
	c.factories = make(map[string]func() interface{})
	c.singletons = make(map[string]interface{})
}

// ServiceNames returns all registered service names
func (c *Container) ServiceNames() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	names := make([]string, 0)

	for name := range c.services {
		names = append(names, name)
	}

	for name := range c.factories {
		names = append(names, name)
	}

	return names
}

// Global container instance
var globalContainer = NewContainer()

// Global functions for convenience

// Register registers a service in the global container
func Register(name string, service interface{}) {
	globalContainer.Register(name, service)
}

// RegisterFactory registers a factory in the global container
func RegisterFactory(name string, factory func() interface{}) {
	globalContainer.RegisterFactory(name, factory)
}

// RegisterSingleton registers a singleton in the global container
func RegisterSingleton(name string, factory func() interface{}) {
	globalContainer.RegisterSingleton(name, factory)
}

// Get retrieves a service from the global container
func Get(name string) (interface{}, error) {
	return globalContainer.Get(name)
}

// MustGet retrieves a service from the global container and panics if not found
func MustGet(name string) interface{} {
	return globalContainer.MustGet(name)
}

// GetTyped retrieves a typed service from the global container
func GetTyped(name string, target interface{}) error {
	return globalContainer.GetTyped(name, target)
}

// Inject performs dependency injection using the global container
func Inject(target interface{}) error {
	return globalContainer.Inject(target)
}

// Has checks if a service exists in the global container
func Has(name string) bool {
	return globalContainer.Has(name)
}

// ServiceProvider defines an interface for service providers
type ServiceProvider interface {
	Register(container *Container) error
	Boot(container *Container) error
}

// RegisterProvider registers a service provider
func (c *Container) RegisterProvider(provider ServiceProvider) error {
	if err := provider.Register(c); err != nil {
		return fmt.Errorf("failed to register provider: %w", err)
	}

	if err := provider.Boot(c); err != nil {
		return fmt.Errorf("failed to boot provider: %w", err)
	}

	return nil
}

// RegisterProvider registers a service provider in the global container
func RegisterProvider(provider ServiceProvider) error {
	return globalContainer.RegisterProvider(provider)
}
