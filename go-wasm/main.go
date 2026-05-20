//go:build js && wasm

package main

import (
	"syscall/js"
	"viewer/renderer"
)

func initRenderer(
	this js.Value,
	args []js.Value,
) any {

	if len(args) < 1 {
		return "missing canvas id"
	}

	return renderer.Init(
		args[0].String(),
	)
}

func loadModel(
	this js.Value,
	args []js.Value,
) any {

	if len(args) < 1 {
		println("missing model data")
		return nil
	}

	uint8Array := args[0]

	length := uint8Array.
		Get("length").
		Int()

	data := make([]byte, length)

	js.CopyBytesToGo(
		data,
		uint8Array,
	)

	err := renderer.LoadModelFromBytes(data)

	if err != nil {

		println(
			"failed to load model:",
			err.Error(),
		)

		return nil
	}

	println("model loaded from ui")

	return nil
}

func registerCallbacks() {

	js.Global().Set(
		"goInitRenderer",
		js.FuncOf(initRenderer),
	)

	js.Global().Set(
		"goHandleMouseMove",
		js.FuncOf(renderer.HandleMouseMove),
	)

	js.Global().Set(
		"goLoadModel",
		js.FuncOf(loadModel),
	)
}

func main() {

	registerCallbacks()

	println("Go WASM initialized")

	select {}
}
