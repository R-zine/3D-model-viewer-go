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

	return renderer.Init(
		args[0].String(),
	)
}

func registerCallbacks() {

	js.Global().Set(
		"goInitRenderer",
		js.FuncOf(initRenderer),
	)
}

func main() {

	registerCallbacks()

	select {}
}