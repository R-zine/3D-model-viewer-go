package main

import (
	"fmt"
	"io"
	"os"
	"viewer/gltf"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	return runWithLimits(args, stdout, stderr, gltf.DefaultLimits())
}

func runWithLimits(args []string, stdout, stderr io.Writer, limits gltf.Limits) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: gltfcheck path/to/model.glb")
		return 2
	}
	path := args[0]
	info, err := os.Stat(path)
	if err != nil {
		return invalid(stderr, err)
	}
	if info.Size() > int64(limits.MaxFileBytes) {
		return invalid(stderr, fmt.Errorf("file exceeds the %d-byte parser limit", limits.MaxFileBytes))
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return invalid(stderr, err)
	}
	model, err := gltf.ParseGLBWithLimits(data, limits)
	if err != nil {
		return invalid(stderr, err)
	}
	vertexCount := 0
	indexCount := 0
	for _, primitive := range model.Primitives {
		vertexCount += len(primitive.Positions) / 3
		indexCount += len(primitive.Indices)
	}
	fmt.Fprintf(stdout,
		"valid GLB: %d primitives, %d vertices, %d indices, %d base-color textures\n",
		len(model.Primitives), vertexCount, indexCount, len(model.Textures),
	)
	return 0
}

func invalid(stderr io.Writer, err error) int {
	fmt.Fprintln(stderr, "invalid GLB:", err)
	return 1
}
