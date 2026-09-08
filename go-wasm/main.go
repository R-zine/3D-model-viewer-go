//go:build js && wasm

package main

import (
	"fmt"
	"math"
	"syscall/js"
	"viewer/renderer"
)

const maxUploadBytes = 128 << 20

var callbacks []js.Func

func main() {
	register("goInitRenderer", initRenderer)
	register("goRotateRenderer", rotateRenderer)
	register("goLoadModel", loadModel)
	register("goRendererFPS", rendererFPS)
	register("goDisposeRenderer", disposeRenderer)
	select {}
}

func register(name string, callback func(js.Value, []js.Value) any) {
	function := js.FuncOf(callback)
	callbacks = append(callbacks, function)
	js.Global().Set(name, function)
}

func initRenderer(this js.Value, args []js.Value) any {
	if len(args) != 1 || args[0].Type() != js.TypeString {
		return "a canvas id is required"
	}
	return errorResult(renderer.Init(args[0].String()))
}

func rotateRenderer(this js.Value, args []js.Value) any {
	if len(args) != 2 || args[0].Type() != js.TypeNumber || args[1].Type() != js.TypeNumber {
		return "two numeric pointer deltas are required"
	}
	deltaX, deltaY := args[0].Float(), args[1].Float()
	if math.IsNaN(deltaX) || math.IsInf(deltaX, 0) || math.IsNaN(deltaY) || math.IsInf(deltaY, 0) {
		return "pointer deltas must be finite"
	}
	renderer.Rotate(deltaX, deltaY)
	return ""
}

func loadModel(this js.Value, args []js.Value) any {
	if len(args) != 1 || !args[0].InstanceOf(js.Global().Get("Uint8Array")) {
		return "model data must be a Uint8Array"
	}
	array := args[0]
	length := array.Get("byteLength").Int()
	if length <= 0 {
		return "model data is empty"
	}
	if length > maxUploadBytes {
		return fmt.Sprintf("model exceeds the %d MiB upload limit", maxUploadBytes>>20)
	}
	data := make([]byte, length)
	if copied := js.CopyBytesToGo(data, array); copied != length {
		return "could not copy all model data into WebAssembly memory"
	}
	return errorResult(renderer.LoadModelFromBytes(data))
}

func rendererFPS(this js.Value, args []js.Value) any {
	return renderer.FPS()
}

func disposeRenderer(this js.Value, args []js.Value) any {
	renderer.Dispose()
	return nil
}

func errorResult(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}
