//go:build js && wasm

package renderer

import (
	"math"
	"syscall/js"
)

var (
	GL js.Value

	program js.Value

	currentMeshes []*GPUMesh

	positionLoc js.Value
	normalLoc   js.Value
	uvLoc       js.Value
	texLoc      js.Value
	lightDirLoc js.Value

	rotationX float64
	rotationY float64
)

func HandleMouseMove(this js.Value, args []js.Value) any {
	if len(args) < 2 {
		return nil
	}

	rotationY += args[0].Float() * 0.01
	rotationX += args[1].Float() * 0.01
	return nil
}

func compileShader(source string, shaderType int) js.Value {
	shader := GL.Call("createShader", shaderType)
	GL.Call("shaderSource", shader, source)
	GL.Call("compileShader", shader)

	if !GL.Call("getShaderParameter", shader, GL.Get("COMPILE_STATUS")).Bool() {
		println(GL.Call("getShaderInfoLog", shader).String())
	}

	return shader
}

func createProgram(vs, fs js.Value) js.Value {
	p := GL.Call("createProgram")
	GL.Call("attachShader", p, vs)
	GL.Call("attachShader", p, fs)
	GL.Call("linkProgram", p)

	if !GL.Call("getProgramParameter", p, GL.Get("LINK_STATUS")).Bool() {
		println(GL.Call("getProgramInfoLog", p).String())
	}

	return p
}

func LoadModelFromBytes(data []byte) error {

	mesh, err := parseGLBMesh(data)
	if err != nil {
		return err
	}

	currentMeshes = nil

	for i := range mesh.Primitives {

		gpuMesh := UploadPrimitiveMesh(
			&mesh.Primitives[i],
		)

		currentMeshes = append(
			currentMeshes,
			gpuMesh,
		)
	}

	println(
		"model loaded:",
		len(currentMeshes),
		"primitives",
	)

	return nil
}

func Init(canvasID string) string {
	document := js.Global().Get("document")
	canvas := document.Call("getElementById", canvasID)

	GL = canvas.Call("getContext", "webgl2")
	if GL.IsNull() {
		return "webgl2 unsupported"
	}

	vs := compileShader(BasicVertexShader, GL.Get("VERTEX_SHADER").Int())
	fs := compileShader(BasicFragmentShader, GL.Get("FRAGMENT_SHADER").Int())

	program = createProgram(vs, fs)
	GL.Call("useProgram", program)

	GL.Call("enable", GL.Get("DEPTH_TEST"))
	GL.Call("enable", GL.Get("CULL_FACE"))
	GL.Call("cullFace", GL.Get("BACK"))

	// Cache attribute/uniform locations ONCE
	positionLoc = GL.Call("getAttribLocation", program, "position")
	normalLoc = GL.Call("getAttribLocation", program, "normal")
	uvLoc = GL.Call("getAttribLocation", program, "uv")
	texLoc = GL.Call("getUniformLocation", program, "diffuseTex")
	lightDirLoc = GL.Call(
		"getUniformLocation",
		program,
		"lightDir",
	)

	LoadGLBMeshAsync("/models/model.glb", func(mesh *Mesh) {
		currentMeshes = nil

		for i := range mesh.Primitives {
			p := UploadPrimitiveMesh(&mesh.Primitives[i])
			currentMeshes = append(currentMeshes, p)
		}

		println("mesh loaded:", len(currentMeshes), "primitives")
	})

	startRenderLoop()
	return "renderer initialized"
}

func setMatrixUniform(name string, m []float32) {
	arr := js.Global().Get("Float32Array").New(len(m))
	for i := range m {
		arr.SetIndex(i, m[i])
	}

	loc := GL.Call("getUniformLocation", program, name)
	GL.Call("uniformMatrix4fv", loc, false, arr)
}

func perspective(fov, aspect, near, far float64) []float32 {
	f := float32(1.0 / math.Tan(fov*0.5*math.Pi/180.0))

	return []float32{
		f / float32(aspect), 0, 0, 0,
		0, f, 0, 0,
		0, 0, float32((far + near) / (near - far)), -1,
		0, 0, float32((2 * far * near) / (near - far)), 0,
	}
}

func renderFrame() {
	if len(currentMeshes) == 0 {
		return
	}

	GL.Call("viewport", 0, 0, 800, 600)

	GL.Call("clearColor", 0.1, 0.1, 0.1, 1.0)
	GL.Call("clear",
		GL.Get("COLOR_BUFFER_BIT").Int()|
			GL.Get("DEPTH_BUFFER_BIT").Int(),
	)

	GL.Call("useProgram", program)

	cx := math.Cos(rotationX)
	sx := math.Sin(rotationX)
	cy := math.Cos(rotationY)
	sy := math.Sin(rotationY)

	model := []float32{

		float32(cy),
		0,
		float32(-sy),
		0,

		float32(sx * sy),
		float32(cx),
		float32(sx * cy),
		0,

		float32(cx * sy),
		float32(-sx),
		float32(cx * cy),
		0,

		0,
		-0.5,
		0,
		1,
	}

	view := []float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, -3, 1,
	}

	projection := perspective(45, 800.0/600.0, 0.1, 100)

	setMatrixUniform("model", model)
	setMatrixUniform("view", view)
	setMatrixUniform("projection", projection)

	// IMPORTANT: set texture unit ALWAYS
	GL.Call("activeTexture", GL.Get("TEXTURE0"))
	GL.Call("uniform1i", texLoc, 0)

	GL.Call(
		"uniform3f",
		lightDirLoc,
		0.5,
		1.0,
		0.8,
	)

	for _, mesh := range currentMeshes {

		GL.Call("bindBuffer", GL.Get("ARRAY_BUFFER"), mesh.VertexBuffer)
		GL.Call("bindBuffer", GL.Get("ELEMENT_ARRAY_BUFFER"), mesh.IndexBuffer)

		stride := 8 * 4

		GL.Call("vertexAttribPointer", positionLoc, 3, GL.Get("FLOAT"), false, stride, 0)
		GL.Call("vertexAttribPointer", normalLoc, 3, GL.Get("FLOAT"), false, stride, 12)
		GL.Call("vertexAttribPointer", uvLoc, 2, GL.Get("FLOAT"), false, stride, 24)

		GL.Call("enableVertexAttribArray", positionLoc)
		GL.Call("enableVertexAttribArray", normalLoc)
		GL.Call("enableVertexAttribArray", uvLoc)

		// Always bind texture (prevents state leakage)
		if mesh.HasTexture {
			GL.Call("bindTexture", GL.Get("TEXTURE_2D"), mesh.Texture)
		} else {
			GL.Call("bindTexture", GL.Get("TEXTURE_2D"), nil)
		}

		GL.Call("drawElements",
			GL.Get("TRIANGLES"),
			mesh.IndexCount,
			GL.Get("UNSIGNED_INT"),
			0,
		)
	}
}

func startRenderLoop() {
	var render js.Func

	render = js.FuncOf(func(this js.Value, args []js.Value) any {
		renderFrame()
		js.Global().Call("requestAnimationFrame", render)
		return nil
	})

	js.Global().Call("requestAnimationFrame", render)
}
