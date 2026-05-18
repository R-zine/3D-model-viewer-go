package renderer

import (
	"math"
	"syscall/js"
)

var (
	GL js.Value

	program js.Value

	rotation float64
)

func compileShader(
	source string,
	shaderType int,
) js.Value {

	shader := GL.Call(
		"createShader",
		shaderType,
	)

	GL.Call(
		"shaderSource",
		shader,
		source,
	)

	GL.Call("compileShader", shader)

	return shader
}

func createProgram(
	vertexShader js.Value,
	fragmentShader js.Value,
) js.Value {

	program := GL.Call("createProgram")

	GL.Call(
		"attachShader",
		program,
		vertexShader,
	)

	GL.Call(
		"attachShader",
		program,
		fragmentShader,
	)

	GL.Call("linkProgram", program)

	return program
}

func Init(
	canvasID string,
) string {

	document := js.Global().Get(
		"document",
	)

	canvas := document.Call(
		"getElementById",
		canvasID,
	)

	GL = canvas.Call(
		"getContext",
		"webgl",
	)

	if GL.IsNull() {
		return "webgl unsupported"
	}

	vertexShader := compileShader(
		BasicVertexShader,
		GL.Get("VERTEX_SHADER").Int(),
	)

	fragmentShader := compileShader(
		BasicFragmentShader,
		GL.Get("FRAGMENT_SHADER").Int(),
	)

	program = createProgram(
		vertexShader,
		fragmentShader,
	)

	GL.Call("useProgram", program)

	GL.Call("enable", GL.Get("DEPTH_TEST"))

	createCubeMesh()

	startRenderLoop()

	return "renderer initialized"
}

func setMatrixUniform(
	name string,
	matrix []float32,
) {

	jsMatrix := js.Global().
		Get("Float32Array").
		New(len(matrix))

	for i, v := range matrix {
		jsMatrix.SetIndex(i, v)
	}

	location := GL.Call(
		"getUniformLocation",
		program,
		name,
	)

	GL.Call(
		"uniformMatrix4fv",
		location,
		false,
		jsMatrix,
	)
}

func perspective(
	fov float64,
	aspect float64,
	near float64,
	far float64,
) []float32 {

	f := float32(
		1.0 / math.Tan(
			fov*0.5*math.Pi/180.0,
		),
	)

	rangeInv := float32(
		1.0 / (near - far),
	)

	return []float32{
		f / float32(aspect), 0, 0, 0,
		0, f, 0, 0,
		0, 0,
		(float32(near)+float32(far))*
			rangeInv,
		-1,
		0, 0,
		2*float32(near)*float32(far)*
			rangeInv,
		0,
	}
}

func renderFrame() {

	rotation += 0.01

	c := math.Cos(rotation)
	s := math.Sin(rotation)

	model := []float32{
		float32(c), 0, float32(s), 0,
		0, 1, 0, 0,
		float32(-s), 0, float32(c), 0,
		0, 0, 0, 1,
	}

	view := []float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, -3, 1,
	}

	projection := perspective(
		45,
		800.0/600.0,
		0.1,
		100,
	)

	stride := 6 * 4

positionLocation := GL.Call(
	"getAttribLocation",
	program,
	"position",
)

GL.Call(
	"vertexAttribPointer",
	positionLocation,
	3,
	GL.Get("FLOAT"),
	false,
	stride,
	0,
)

GL.Call(
	"enableVertexAttribArray",
	positionLocation,
)

normalLocation := GL.Call(
	"getAttribLocation",
	program,
	"normal",
)

GL.Call(
	"vertexAttribPointer",
	normalLocation,
	3,
	GL.Get("FLOAT"),
	false,
	stride,
	3*4,
)

GL.Call(
	"enableVertexAttribArray",
	normalLocation,
)

	setMatrixUniform(
		"model",
		model,
	)

	setMatrixUniform(
		"view",
		view,
	)

	setMatrixUniform(
		"projection",
		projection,
	)

	GL.Call(
		"viewport",
		0,
		0,
		800,
		600,
	)

	GL.Call(
		"clearColor",
		0.1,
		0.1,
		0.1,
		1.0,
	)

	GL.Call(
		"clear",
		GL.Get("COLOR_BUFFER_BIT").
			Int()|
			GL.Get("DEPTH_BUFFER_BIT").
				Int(),
	)

	GL.Call(
		"drawElements",
		GL.Get("TRIANGLES"),
		indexCount,
		GL.Get("UNSIGNED_SHORT"),
		0,
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