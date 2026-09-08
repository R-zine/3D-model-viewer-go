//go:build js && wasm

package renderer

import (
	"encoding/binary"
	"fmt"
	"math"
	"syscall/js"
	"viewer/gltf"
)

type shaderLocations struct {
	position        int
	normal          int
	uv              int
	model           js.Value
	normalMatrix    js.Value
	view            js.Value
	projection      js.Value
	diffuseTexture  js.Value
	baseColorFactor js.Value
	lightDirection  js.Value
	alphaCutoff     js.Value
}

type uniformBuffer struct {
	floats js.Value
	bytes  js.Value
	data   []byte
}

func newUniformBuffer(length int) uniformBuffer {
	buffer := js.Global().Get("ArrayBuffer").New(length * 4)
	return uniformBuffer{
		floats: js.Global().Get("Float32Array").New(buffer),
		bytes:  js.Global().Get("Uint8Array").New(buffer),
		data:   make([]byte, length*4),
	}
}

func (buffer *uniformBuffer) Set(values []float32) {
	for index, value := range values {
		binary.LittleEndian.PutUint32(buffer.data[index*4:index*4+4], math.Float32bits(value))
	}
	js.CopyBytesToJS(buffer.bytes, buffer.data)
}

type Renderer struct {
	canvasID  string
	canvas    js.Value
	gl        js.Value
	program   js.Value
	locations shaderLocations

	model        *GPUModel
	sourceModel  *gltf.Model
	whiteTexture js.Value
	fitTransform gltf.Mat4

	rotationX float64
	rotationY float64

	projectionBuffer uniformBuffer
	viewBuffer       uniformBuffer
	modelBuffer      uniformBuffer
	normalBuffer     uniformBuffer

	renderCallback          js.Func
	frameRequestID          int
	running                 bool
	contextLost             bool
	contextLostCallback     js.Func
	contextRestoredCallback js.Func

	fps            float64
	fpsFrameCount  int
	fpsWindowStart float64
}

var activeRenderer *Renderer

func Init(canvasID string) error {
	if activeRenderer != nil && activeRenderer.running && activeRenderer.canvasID == canvasID {
		return nil
	}
	if activeRenderer != nil {
		activeRenderer.Dispose()
	}
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return fmt.Errorf("document is unavailable")
	}
	canvas := document.Call("getElementById", canvasID)
	if canvas.IsUndefined() || canvas.IsNull() {
		return fmt.Errorf("canvas %q was not found", canvasID)
	}
	if canvas.Get("getContext").Type() != js.TypeFunction {
		return fmt.Errorf("element %q is not a canvas", canvasID)
	}
	gl := canvas.Call("getContext", "webgl2", map[string]any{"alpha": false, "antialias": true})
	if gl.IsUndefined() || gl.IsNull() {
		return fmt.Errorf("WebGL2 is unsupported")
	}
	renderer := &Renderer{
		canvasID:         canvasID,
		canvas:           canvas,
		gl:               gl,
		fitTransform:     gltf.Identity(),
		projectionBuffer: newUniformBuffer(16),
		viewBuffer:       newUniformBuffer(16),
		modelBuffer:      newUniformBuffer(16),
		normalBuffer:     newUniformBuffer(9),
	}
	if err := renderer.initializeGL(); err != nil {
		return err
	}
	renderer.installContextHandlers()
	renderer.start()
	activeRenderer = renderer
	return nil
}

func LoadModelFromBytes(data []byte) error {
	if activeRenderer == nil || !activeRenderer.running {
		return fmt.Errorf("renderer is not initialized")
	}
	source, err := gltf.ParseGLB(data)
	if err != nil {
		return err
	}
	return activeRenderer.Load(source)
}

func Rotate(deltaX, deltaY float64) {
	if activeRenderer == nil {
		return
	}
	activeRenderer.rotationY = math.Mod(activeRenderer.rotationY+deltaX*0.01, math.Pi*2)
	activeRenderer.rotationX = max(-math.Pi/2, min(math.Pi/2, activeRenderer.rotationX+deltaY*0.01))
}

func FPS() float64 {
	if activeRenderer == nil {
		return 0
	}
	return activeRenderer.fps
}

func Dispose() {
	if activeRenderer == nil {
		return
	}
	activeRenderer.Dispose()
	activeRenderer = nil
}

func (renderer *Renderer) initializeGL() error {
	gl := renderer.gl
	vertexShader, err := renderer.compileShader(BasicVertexShader, gl.Get("VERTEX_SHADER").Int())
	if err != nil {
		return err
	}
	fragmentShader, err := renderer.compileShader(BasicFragmentShader, gl.Get("FRAGMENT_SHADER").Int())
	if err != nil {
		gl.Call("deleteShader", vertexShader)
		return err
	}
	program, err := renderer.createProgram(vertexShader, fragmentShader)
	gl.Call("deleteShader", vertexShader)
	gl.Call("deleteShader", fragmentShader)
	if err != nil {
		return err
	}
	renderer.program = program
	gl.Call("useProgram", program)
	gl.Call("enable", gl.Get("DEPTH_TEST"))
	gl.Call("depthFunc", gl.Get("LEQUAL"))
	gl.Call("blendFunc", gl.Get("SRC_ALPHA"), gl.Get("ONE_MINUS_SRC_ALPHA"))

	locations := shaderLocations{
		position:        gl.Call("getAttribLocation", program, "position").Int(),
		normal:          gl.Call("getAttribLocation", program, "normal").Int(),
		uv:              gl.Call("getAttribLocation", program, "uv").Int(),
		model:           gl.Call("getUniformLocation", program, "model"),
		normalMatrix:    gl.Call("getUniformLocation", program, "normalMatrix"),
		view:            gl.Call("getUniformLocation", program, "view"),
		projection:      gl.Call("getUniformLocation", program, "projection"),
		diffuseTexture:  gl.Call("getUniformLocation", program, "diffuseTex"),
		baseColorFactor: gl.Call("getUniformLocation", program, "baseColorFactor"),
		lightDirection:  gl.Call("getUniformLocation", program, "lightDir"),
		alphaCutoff:     gl.Call("getUniformLocation", program, "alphaCutoff"),
	}
	if locations.position < 0 || locations.normal < 0 || locations.uv < 0 {
		gl.Call("deleteProgram", program)
		return fmt.Errorf("shader program is missing a required vertex attribute")
	}
	for name, location := range map[string]js.Value{
		"model": locations.model, "normalMatrix": locations.normalMatrix, "view": locations.view,
		"projection": locations.projection, "diffuseTex": locations.diffuseTexture,
		"baseColorFactor": locations.baseColorFactor, "lightDir": locations.lightDirection,
		"alphaCutoff": locations.alphaCutoff,
	} {
		if location.IsNull() {
			gl.Call("deleteProgram", program)
			return fmt.Errorf("shader program is missing uniform %s", name)
		}
	}
	renderer.locations = locations
	whiteTexture, err := renderer.createWhiteTexture()
	if err != nil {
		gl.Call("deleteProgram", program)
		return err
	}
	renderer.whiteTexture = whiteTexture
	return nil
}

func (renderer *Renderer) compileShader(source string, shaderType int) (js.Value, error) {
	shader := renderer.gl.Call("createShader", shaderType)
	if shader.IsNull() {
		return js.Undefined(), fmt.Errorf("WebGL could not allocate a shader")
	}
	renderer.gl.Call("shaderSource", shader, source)
	renderer.gl.Call("compileShader", shader)
	if !renderer.gl.Call("getShaderParameter", shader, renderer.gl.Get("COMPILE_STATUS")).Bool() {
		message := renderer.gl.Call("getShaderInfoLog", shader).String()
		renderer.gl.Call("deleteShader", shader)
		return js.Undefined(), fmt.Errorf("compile shader: %s", message)
	}
	return shader, nil
}

func (renderer *Renderer) createProgram(vertexShader, fragmentShader js.Value) (js.Value, error) {
	program := renderer.gl.Call("createProgram")
	if program.IsNull() {
		return js.Undefined(), fmt.Errorf("WebGL could not allocate a shader program")
	}
	renderer.gl.Call("attachShader", program, vertexShader)
	renderer.gl.Call("attachShader", program, fragmentShader)
	renderer.gl.Call("linkProgram", program)
	if !renderer.gl.Call("getProgramParameter", program, renderer.gl.Get("LINK_STATUS")).Bool() {
		message := renderer.gl.Call("getProgramInfoLog", program).String()
		renderer.gl.Call("deleteProgram", program)
		return js.Undefined(), fmt.Errorf("link shader program: %s", message)
	}
	return program, nil
}

func (renderer *Renderer) Load(source *gltf.Model) error {
	model, err := renderer.uploadModel(source)
	if err != nil {
		return err
	}
	previous := renderer.model
	renderer.model = model
	renderer.sourceModel = source
	renderer.fitTransform = fitToView(source.Bounds)
	renderer.rotationX = 0
	renderer.rotationY = 0
	previous.Dispose(renderer.gl)
	return nil
}

func (renderer *Renderer) start() {
	if renderer.running {
		return
	}
	renderer.running = true
	renderer.renderCallback = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !renderer.running {
			return nil
		}
		timestamp := 0.0
		if len(args) > 0 {
			timestamp = args[0].Float()
		}
		renderer.renderFrame(timestamp)
		renderer.frameRequestID = js.Global().Call("requestAnimationFrame", renderer.renderCallback).Int()
		return nil
	})
	renderer.frameRequestID = js.Global().Call("requestAnimationFrame", renderer.renderCallback).Int()
}

func (renderer *Renderer) renderFrame(timestamp float64) {
	if renderer.contextLost {
		return
	}
	gl := renderer.gl
	width := gl.Get("drawingBufferWidth").Int()
	height := gl.Get("drawingBufferHeight").Int()
	if width <= 0 || height <= 0 {
		return
	}
	gl.Call("viewport", 0, 0, width, height)
	gl.Call("clearColor", 0.06, 0.075, 0.09, 1)
	gl.Call("clear", gl.Get("COLOR_BUFFER_BIT").Int()|gl.Get("DEPTH_BUFFER_BIT").Int())
	if renderer.model == nil || len(renderer.model.Meshes) == 0 {
		renderer.updateFPS(timestamp, false)
		return
	}

	gl.Call("useProgram", renderer.program)
	projection := perspective(45, float64(width)/float64(height), 0.1, 100)
	view := gltf.Mat4{1, 0, 0, 0, 0, 1, 0, 0, 0, 0, 1, 0, 0, 0, -3, 1}
	interaction := multiplyMatrices(rotationMatrix(renderer.rotationX, renderer.rotationY), renderer.fitTransform)
	renderer.setMatrix4(renderer.locations.projection, &renderer.projectionBuffer, projection)
	renderer.setMatrix4(renderer.locations.view, &renderer.viewBuffer, view)
	gl.Call("activeTexture", gl.Get("TEXTURE0"))
	gl.Call("uniform1i", renderer.locations.diffuseTexture, 0)
	gl.Call("uniform3f", renderer.locations.lightDirection, 0.5, 1.0, 0.8)

	for _, mesh := range renderer.model.Meshes {
		modelMatrix := multiplyMatrices(interaction, mesh.Transform)
		renderer.setMatrix4(renderer.locations.model, &renderer.modelBuffer, modelMatrix)
		renderer.setMatrix3(renderer.locations.normalMatrix, &renderer.normalBuffer, normalMatrix(modelMatrix))
		factor := mesh.Material.BaseColorFactor
		gl.Call("uniform4f", renderer.locations.baseColorFactor, factor[0], factor[1], factor[2], factor[3])
		if mesh.Material.AlphaMode == "MASK" {
			gl.Call("uniform1f", renderer.locations.alphaCutoff, mesh.Material.AlphaCutoff)
		} else {
			gl.Call("uniform1f", renderer.locations.alphaCutoff, -1)
		}
		if mesh.Material.DoubleSided {
			gl.Call("disable", gl.Get("CULL_FACE"))
		} else {
			gl.Call("enable", gl.Get("CULL_FACE"))
			gl.Call("cullFace", gl.Get("BACK"))
		}
		if mesh.Material.AlphaMode == "BLEND" {
			gl.Call("enable", gl.Get("BLEND"))
			gl.Call("depthMask", false)
		} else {
			gl.Call("disable", gl.Get("BLEND"))
			gl.Call("depthMask", true)
		}
		texture := renderer.whiteTexture
		if textureIndex := mesh.Material.BaseColorTexture; textureIndex >= 0 && textureIndex < len(renderer.model.Textures) {
			texture = renderer.model.Textures[textureIndex].value
		}
		gl.Call("bindTexture", gl.Get("TEXTURE_2D"), texture)
		gl.Call("bindVertexArray", mesh.VertexArray)
		gl.Call("drawElements", mesh.Mode, mesh.IndexCount, gl.Get("UNSIGNED_INT"), 0)
	}
	gl.Call("bindVertexArray", nil)
	gl.Call("depthMask", true)
	renderer.updateFPS(timestamp, true)
}

func (renderer *Renderer) updateFPS(timestamp float64, drew bool) {
	if renderer.fpsWindowStart == 0 {
		renderer.fpsWindowStart = timestamp
	}
	if drew {
		renderer.fpsFrameCount++
	}
	elapsed := timestamp - renderer.fpsWindowStart
	if elapsed >= 1000 {
		renderer.fps = float64(renderer.fpsFrameCount) * 1000 / elapsed
		renderer.fpsFrameCount = 0
		renderer.fpsWindowStart = timestamp
	}
}

func (renderer *Renderer) setMatrix4(location js.Value, buffer *uniformBuffer, matrix gltf.Mat4) {
	buffer.Set(matrix[:])
	renderer.gl.Call("uniformMatrix4fv", location, false, buffer.floats)
}

func (renderer *Renderer) setMatrix3(location js.Value, buffer *uniformBuffer, matrix [9]float32) {
	buffer.Set(matrix[:])
	renderer.gl.Call("uniformMatrix3fv", location, false, buffer.floats)
}

func (renderer *Renderer) installContextHandlers() {
	renderer.contextLostCallback = js.FuncOf(func(this js.Value, args []js.Value) any {
		if len(args) > 0 {
			args[0].Call("preventDefault")
		}
		renderer.contextLost = true
		renderer.fps = 0
		renderer.fpsFrameCount = 0
		renderer.fpsWindowStart = 0
		renderer.model.Dispose(renderer.gl)
		renderer.model = nil
		dispatchRendererError("WebGL context was lost; waiting for restoration")
		return nil
	})
	renderer.contextRestoredCallback = js.FuncOf(func(this js.Value, args []js.Value) any {
		renderer.contextLost = false
		if err := renderer.initializeGL(); err != nil {
			renderer.contextLost = true
			dispatchRendererError(err.Error())
			return nil
		}
		if renderer.sourceModel != nil {
			model, err := renderer.uploadModel(renderer.sourceModel)
			if err != nil {
				renderer.contextLost = true
				dispatchRendererError(err.Error())
				return nil
			}
			renderer.model = model
		}
		return nil
	})
	renderer.canvas.Call("addEventListener", "webglcontextlost", renderer.contextLostCallback)
	renderer.canvas.Call("addEventListener", "webglcontextrestored", renderer.contextRestoredCallback)
}

func (renderer *Renderer) Dispose() {
	if !renderer.running {
		return
	}
	renderer.running = false
	js.Global().Call("cancelAnimationFrame", renderer.frameRequestID)
	renderer.canvas.Call("removeEventListener", "webglcontextlost", renderer.contextLostCallback)
	renderer.canvas.Call("removeEventListener", "webglcontextrestored", renderer.contextRestoredCallback)
	renderer.contextLostCallback.Release()
	renderer.contextRestoredCallback.Release()
	renderer.renderCallback.Release()
	renderer.model.Dispose(renderer.gl)
	renderer.model = nil
	renderer.sourceModel = nil
	if renderer.whiteTexture.Truthy() {
		renderer.gl.Call("deleteTexture", renderer.whiteTexture)
	}
	if renderer.program.Truthy() {
		renderer.gl.Call("deleteProgram", renderer.program)
	}
}
