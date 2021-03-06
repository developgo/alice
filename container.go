// Package alice provides an additive dependency injection container.
package alice

import (
	"fmt"
	"reflect"
)

// CreateContainer creates a new instance of container with specified modules. It panics if any of the module is
// invalid. This is the only way to create a container. Most applications call it only once during bootstrap.
func CreateContainer(modules ...Module) Container {
	c := &container{
		modules: modules,
	}
	c.populate()
	return c
}

// Container defines the interface of an instance container. It initializes instances based on dependencies,
// and provides APIs to retrieve instances by type or name.
type Container interface {
	// Instance returns an instance by type. It panics when no instance is found,
	// or multiple instances are found for the same type.
	Instance(t reflect.Type) interface{}
	// InstanceByName returns an instance by name. It panics when no instance is found.
	InstanceByName(name string) interface{}
}

// container is an implementation of Container interface. It is not thread-safe.
type container struct {
	modules []Module

	instanceByName map[string]interface{}
	instanceByType map[reflect.Type][]interface{}
}

func (c *container) Instance(t reflect.Type) interface{} {
	return c.findInstanceByType(t)
}

func (c *container) InstanceByName(name string) interface{} {
	return c.findInstanceByName(name)
}

func (c *container) populate() {
	rms := c.reflectModules(c.modules)
	g, err := createGraph(rms...)
	if err != nil {
		panic(err)
	}

	orderedRms, err := g.instantiationOrder()
	if err != nil {
		panic(err)
	}

	c.instanceByName = make(map[string]interface{})
	c.instanceByType = make(map[reflect.Type][]interface{})
	for _, rm := range orderedRms {
		c.instantiateModule(rm)
	}
}

func (c *container) instantiateModule(rm *reflectedModule) {
	for _, dep := range rm.namedDepends {
		instance := c.findInstanceByName(dep.name)
		dep.field.Set(reflect.ValueOf(instance))
	}
	for _, dep := range rm.typedDepends {
		instance := c.findInstanceByType(dep.tp)
		dep.field.Set(reflect.ValueOf(instance))
	}

	for _, instanceMethod := range rm.instances {
		instance := instanceMethod.method.Call(nil)[0].Interface()

		c.instanceByName[instanceMethod.name] = instance

		typedInstances, _ := c.instanceByType[instanceMethod.tp]
		typedInstances = append(typedInstances, instance)
		c.instanceByType[instanceMethod.tp] = typedInstances
	}
}

func (c *container) findInstanceByType(t reflect.Type) interface{} {
	instances, ok := c.instanceByType[t]
	if !ok {
		instances = c.findAssignableInstances(t)
	}
	if len(instances) == 0 {
		panic(fmt.Sprintf("instance type %s is not defined", t.Name()))
	}
	if len(instances) > 1 {
		panic(fmt.Sprintf("instance type %s has more than one instances defined", t.Name()))
	}

	return instances[0]
}

func (c *container) findInstanceByName(name string) interface{} {
	instance, ok := c.instanceByName[name]
	if !ok {
		panic(fmt.Sprintf("instance name %s is not defined", name))
	}
	return instance
}

func (c *container) findAssignableInstances(t reflect.Type) []interface{} {
	var instances []interface{}
	for _, instance := range c.instanceByName {
		instanceType := reflect.TypeOf(instance)
		if instanceType.AssignableTo(t) {
			instances = append(instances, instance)
		}
	}
	return instances
}

func (c *container) reflectModules(modules []Module) []*reflectedModule {
	var rms []*reflectedModule
	for _, m := range c.modules {
		rm, err := reflectModule(m)
		if err != nil {
			panic(err)
		}
		rms = append(rms, rm)
	}
	return rms
}
