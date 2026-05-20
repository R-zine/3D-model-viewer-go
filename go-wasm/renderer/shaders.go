package renderer

import _ "embed"

//go:embed shaders/basic.vert
var BasicVertexShader string

//go:embed shaders/basic.frag
var BasicFragmentShader string
