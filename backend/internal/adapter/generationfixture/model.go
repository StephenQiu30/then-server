package generationfixture

import (
	"bytes"
	"image"
	"image/color"
	"image/png"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

// placeholderGLB is a textured triangle used only to exercise model task
// storage and validation. It does not represent a person or outfit.
func placeholderGLB() ([]byte, error) {
	texture := image.NewRGBA(image.Rect(0, 0, 2, 2))
	texture.SetRGBA(0, 0, color.RGBA{R: 180, G: 80, B: 45, A: 255})
	var pngBytes bytes.Buffer
	if err := png.Encode(&pngBytes, texture); err != nil {
		return nil, err
	}
	doc := gltf.NewDocument()
	positions := modeler.WritePosition(doc, [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}})
	indices := modeler.WriteIndices(doc, []uint16{0, 1, 2})
	coords := modeler.WriteTextureCoord(doc, [][2]float32{{0, 0}, {1, 0}, {0, 1}})
	imageIndex, err := modeler.WriteImage(doc, "fixture-texture", "image/png", bytes.NewReader(pngBytes.Bytes()))
	if err != nil {
		return nil, err
	}
	doc.Textures = []*gltf.Texture{{Source: gltf.Index(imageIndex)}}
	doc.Materials = []*gltf.Material{{PBRMetallicRoughness: &gltf.PBRMetallicRoughness{
		BaseColorTexture: &gltf.TextureInfo{Index: 0},
	}}}
	doc.Meshes = []*gltf.Mesh{{Primitives: []*gltf.Primitive{{
		Indices:    gltf.Index(indices),
		Attributes: gltf.PrimitiveAttributes{gltf.POSITION: positions, gltf.TEXCOORD_0: coords},
		Material:   gltf.Index(0),
	}}}}
	doc.Nodes = []*gltf.Node{{Mesh: gltf.Index(0)}}
	doc.Scenes[0].Nodes = []int{0}
	var output bytes.Buffer
	if err := gltf.NewEncoder(&output).Encode(doc); err != nil {
		return nil, err
	}
	return output.Bytes(), nil
}
