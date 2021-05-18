package main

import (
	wasmer "github.com/wasmerio/wasmer-go/wasmer"
	"k8s.io/klog"
)

func execWasm(wasmBytes []byte, functionName string) error {
	engine := wasmer.NewEngine()
	store := wasmer.NewStore(engine)

	// Compiles the module
	module, err := wasmer.NewModule(store, wasmBytes)
	if err != nil {
		return err
	}

	// Instantiates the module
	importObject := wasmer.NewImportObject()
	instance, err := wasmer.NewInstance(module, importObject)
	if err != nil {
		return err
	}

	// Gets the exported function from the WebAssembly instance.
	fn, err := instance.Exports.GetFunction(functionName)
	if err != nil {
		return err
	}

	// Calls that exported function with Go standard values. The WebAssembly
	// types are inferred and values are casted automatically.
	result, err := fn(1, 9)
	if err != nil {
		return err
	}

	klog.Error(result)
	return nil
}
