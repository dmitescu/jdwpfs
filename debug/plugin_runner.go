// SPDX-License-Identifier: LGPL-3.0
// Copyright (C) 2022 jdwpfs Authors M. G. Dan

package debug

import (
	"os"
	"fmt"
	"plugin"

	jdwp "github.com/omerye/gojdb/jdwp"
)

const (
	PluginEntrypoint = "JdwpfsPluginEntrypoint"
)

//
// Plugin error
//
type PluginBuilderError struct {
	err error
	message string
}

func (e PluginBuilderError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("plugin builder error: %s", e.err)
	}

	return fmt.Sprintf("plugin builder error: %s", e.message)
}

type PluginError struct {
	err error
	message string
}

func (e PluginError) Error() string {
	if e.err != nil {
		return fmt.Sprintf("plugin error: %s", e.err)
	}

	return fmt.Sprintf("plugin error: %s", e.message)
}

type PluginErrors struct {
	errors []PluginError
}

func (e PluginErrors) Error() string {
	var stringResult = ""

	stringResult = fmt.Sprintf("unable to successfully consume events: ")
	for _, err := range e.errors {
		stringResult = fmt.Sprintf("%s\n%s", stringResult, err.Error())
	}

	return stringResult
}

func NewPluginErrors() PluginErrors {
	return PluginErrors {
		errors: []PluginError{},
	}
}

func (e *PluginErrors) AddError(err PluginError) {
	e.errors = append(e.errors, err)
}

func (e *PluginErrors) HasErrors() bool {
	return len(e.errors) != 0
}

//
// PluginInstance
//
type PluginInstance struct {
	name string
	pluginPath string
	plugin *plugin.Plugin
	entrypoint func(string, jdwp.Event) error
}

//
// PluginRunner
//
type PluginRunner struct {
	plugins []*PluginInstance
}

func (r PluginRunner) Entrypoint(event jdwp.Event) error {
	var finalResult = NewPluginErrors()

	for _, pluginInstance := range r.plugins {
		err := pluginInstance.entrypoint(pluginInstance.name, event)
		if err != nil {
			pluginErr := PluginError {
				message: "error processing plugin",
				err: err,
			}
			finalResult.AddError(pluginErr)
		}
	}

	if finalResult.HasErrors() {
		return finalResult
	}

	return nil
}

//
// PluginRunnerBuilder
//
type PluginRunnerBuilder struct {
	pluginPaths map[string]string
}

func NewPluginRunnerBuilder() *PluginRunnerBuilder {
	return &PluginRunnerBuilder {
		pluginPaths: map[string]string{},
	}
}

func (b *PluginRunnerBuilder) AddLocation(name, location string) error {
	_, err := os.Stat(location)
	if err != nil {
		return PluginBuilderError{ message: "unable to find plugin file", err: err }
	}

	if _, ok := b.pluginPaths[name]; ok {
		return PluginBuilderError{ message: "plugin already registered", err: nil }
	}

	b.pluginPaths[name] = location
	
	return nil
}

func (b *PluginRunnerBuilder) Build() (*PluginRunner, error) {
	var newInstances = []*PluginInstance {}
	
	for pluginName, pluginPath := range b.pluginPaths {
		newPlugin, err := plugin.Open(pluginPath)
		if err != nil {
			return nil, PluginBuilderError{ message: "unable to open plugin", err: err }
		}

		entrypointSymbol, err := newPlugin.Lookup(PluginEntrypoint)
		if err != nil {
			return nil, PluginBuilderError{ message: "unable to find symbol in plugin", err: err }
		}

		entrypoint := entrypointSymbol.(func(string, jdwp.Event) error)

		newInstance := &PluginInstance {
			name: pluginName,
			pluginPath: pluginPath,
			plugin: newPlugin,
			entrypoint: entrypoint,
		}

		newInstances = append(newInstances, newInstance)
	}

	newRunner := &PluginRunner {
		plugins: newInstances,
	}

	return newRunner, nil
}
