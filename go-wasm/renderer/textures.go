//go:build js && wasm

package renderer

import (
	"fmt"
	"syscall/js"
	"viewer/gltf"
)

type GPUTexture struct {
	value    js.Value
	image    js.Value
	url      string
	onload   js.Func
	onerror  js.Func
	pending  bool
	disposed bool
}

func (texture *GPUTexture) Dispose(gl js.Value) {
	if texture == nil || texture.disposed {
		return
	}
	texture.disposed = true
	texture.releaseCallbacks()
	if texture.value.Truthy() {
		gl.Call("deleteTexture", texture.value)
	}
}

func (texture *GPUTexture) releaseCallbacks() {
	if !texture.pending {
		return
	}
	texture.pending = false
	if texture.image.Truthy() {
		texture.image.Set("onload", nil)
		texture.image.Set("onerror", nil)
		texture.image = js.Undefined()
	}
	if texture.url != "" {
		js.Global().Get("URL").Call("revokeObjectURL", texture.url)
		texture.url = ""
	}
	texture.onload.Release()
	texture.onerror.Release()
}

func (renderer *Renderer) createWhiteTexture() (js.Value, error) {
	gl := renderer.gl
	texture := gl.Call("createTexture")
	if texture.IsNull() {
		return js.Undefined(), fmt.Errorf("WebGL could not allocate the fallback texture")
	}
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), texture)
	pixel := bytesToJS([]byte{255, 255, 255, 255})
	gl.Call("texImage2D", gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"), 1, 1, 0, gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), pixel)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MIN_FILTER"), gl.Get("LINEAR"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MAG_FILTER"), gl.Get("LINEAR"))
	return texture, nil
}

func (renderer *Renderer) uploadTexture(source gltf.Texture, index int) (*GPUTexture, error) {
	gl := renderer.gl
	value := gl.Call("createTexture")
	if value.IsNull() {
		return nil, fmt.Errorf("WebGL could not allocate texture %d", index)
	}
	gl.Call("bindTexture", gl.Get("TEXTURE_2D"), value)
	gl.Call("texImage2D", gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"), 1, 1, 0, gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), bytesToJS([]byte{255, 255, 255, 255}))
	gl.Call("generateMipmap", gl.Get("TEXTURE_2D"))
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MIN_FILTER"), source.Sampler.MinFilter)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_MAG_FILTER"), source.Sampler.MagFilter)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_S"), source.Sampler.WrapS)
	gl.Call("texParameteri", gl.Get("TEXTURE_2D"), gl.Get("TEXTURE_WRAP_T"), source.Sampler.WrapT)

	array := bytesToJS(source.Data)
	blob := js.Global().Get("Blob").New([]any{array}, map[string]any{"type": source.MIMEType})
	url := js.Global().Get("URL").Call("createObjectURL", blob).String()
	image := js.Global().Get("Image").New()
	texture := &GPUTexture{value: value, image: image, url: url, pending: true}

	texture.onload = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !texture.disposed {
			width := image.Get("naturalWidth").Int()
			height := image.Get("naturalHeight").Int()
			maxSize := gl.Call("getParameter", gl.Get("MAX_TEXTURE_SIZE")).Int()
			if width <= 0 || height <= 0 || width > maxSize || height > maxSize || int64(width)*int64(height) > 64_000_000 {
				dispatchRendererError(fmt.Sprintf("Texture %d dimensions are invalid or too large; using a white fallback", index))
			} else {
				gl.Call("bindTexture", gl.Get("TEXTURE_2D"), value)
				gl.Call("pixelStorei", gl.Get("UNPACK_FLIP_Y_WEBGL"), false)
				gl.Call("texImage2D", gl.Get("TEXTURE_2D"), 0, gl.Get("RGBA"), gl.Get("RGBA"), gl.Get("UNSIGNED_BYTE"), image)
				gl.Call("generateMipmap", gl.Get("TEXTURE_2D"))
			}
		}
		texture.releaseCallbacks()
		return nil
	})
	texture.onerror = js.FuncOf(func(this js.Value, args []js.Value) any {
		if !texture.disposed {
			dispatchRendererError(fmt.Sprintf("Texture %d could not be decoded; using a white fallback", index))
		}
		texture.releaseCallbacks()
		return nil
	})
	image.Set("onload", texture.onload)
	image.Set("onerror", texture.onerror)
	image.Set("src", url)
	return texture, nil
}

func dispatchRendererError(message string) {
	document := js.Global().Get("document")
	if document.IsUndefined() || document.IsNull() {
		return
	}
	event := js.Global().Get("CustomEvent").New("renderer-error", map[string]any{"detail": message})
	document.Call("dispatchEvent", event)
}
