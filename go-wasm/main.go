package main

import (
	"math"
	"syscall/js"
)

var (
	gl js.Value

	program js.Value

	positionBuffer js.Value

	rotation float64
)

func compileShader(
	source string,
	shaderType int,
) js.Value {

	shader := gl.Call(
		"createShader",
		shaderType,
	)

	gl.Call(
		"shaderSource",
		shader,
		source,
	)

	gl.Call("compileShader", shader)

	return shader
}

func createProgram(
	vertexShader js.Value,
	fragmentShader js.Value,
) js.Value {

	program := gl.Call("createProgram")

	gl.Call(
		"attachShader",
		program,
		vertexShader,
	)

	gl.Call(
		"attachShader",
		program,
		fragmentShader,
	)

	gl.Call("linkProgram", program)

	return program
}

func initRenderer(
	this js.Value,
	args []js.Value,
) any {

	canvasID := args[0].String()

	document := js.Global().Get("document")

	canvas := document.Call(
		"getElementById",
		canvasID,
	)

	gl = canvas.Call("getContext", "webgl")

	if gl.IsNull() {
		return "webgl unsupported"
	}

	vertexShaderSource := `
attribute vec4 position;

uniform mat4 model;

void main() {
	gl_Position = model * position;
}
`

	fragmentShaderSource := `
precision mediump float;

void main() {
	gl_FragColor = vec4(
		0.2,
		0.8,
		1.0,
		1.0
	);
}
`

	vertexShader := compileShader(
		vertexShaderSource,
		gl.Get("VERTEX_SHADER").Int(),
	)

	fragmentShader := compileShader(
		fragmentShaderSource,
		gl.Get("FRAGMENT_SHADER").Int(),
	)

	program = createProgram(
		vertexShader,
		fragmentShader,
	)

	gl.Call("useProgram", program)

	vertices := []float32{
		-0.5, -0.5, 0.0,
		0.5, -0.5, 0.0,
		0.0, 0.5, 0.0,
	}

	jsVertices := js.Global().Get(
		"Float32Array",
	).New(len(vertices))

	for i, v := range vertices {
		jsVertices.SetIndex(i, v)
	}

	positionBuffer = gl.Call(
		"createBuffer",
	)

	gl.Call(
		"bindBuffer",
		gl.Get("ARRAY_BUFFER"),
		positionBuffer,
	)

	gl.Call(
		"bufferData",
		gl.Get("ARRAY_BUFFER"),
		jsVertices,
		gl.Get("STATIC_DRAW"),
	)

	positionLocation := gl.Call(
		"getAttribLocation",
		program,
		"position",
	)

	gl.Call(
		"vertexAttribPointer",
		positionLocation,
		3,
		gl.Get("FLOAT"),
		false,
		0,
		0,
	)

	gl.Call(
		"enableVertexAttribArray",
		positionLocation,
	)

	startRenderLoop()

	return "renderer initialized"
}

func renderFrame() {

	rotation += 0.01

	c := math.Cos(rotation)
	s := math.Sin(rotation)

	matrix := []float32{
		float32(c), float32(-s), 0, 0,
		float32(s), float32(c), 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}

	jsMatrix := js.Global().Get(
		"Float32Array",
	).New(len(matrix))

	for i, v := range matrix {
		jsMatrix.SetIndex(i, v)
	}

	modelLocation := gl.Call(
		"getUniformLocation",
		program,
		"model",
	)

	gl.Call(
		"uniformMatrix4fv",
		modelLocation,
		false,
		jsMatrix,
	)

	gl.Call(
		"viewport",
		0,
		0,
		800,
		600,
	)

	gl.Call(
		"clearColor",
		0.1,
		0.1,
		0.1,
		1.0,
	)

	gl.Call(
		"clear",
		gl.Get("COLOR_BUFFER_BIT"),
	)

	gl.Call(
		"drawArrays",
		gl.Get("TRIANGLES"),
		0,
		3,
	)
}

func startRenderLoop() {

	var render js.Func

	render = js.FuncOf(func(
		this js.Value,
		args []js.Value,
	) any {

		renderFrame()

		js.Global().Call(
			"requestAnimationFrame",
			render,
		)

		return nil
	})

	js.Global().Call(
		"requestAnimationFrame",
		render,
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