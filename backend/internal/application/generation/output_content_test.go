package generation

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"math"
	"testing"
)

func TestNormalizeAndVerifyGenerationImage(t *testing.T) {
	input := testJPEG(t)
	metadata := []byte{0xff, 0xe1, 0x00, 0x08, 'e', 'x', 'i', 'f', 1, 2}
	input = append(append(append([]byte(nil), input[:2]...), metadata...), input[2:]...)

	normalized, err := NormalizeGenerationImage(input)
	if err != nil {
		t.Fatalf("NormalizeGenerationImage() error = %v", err)
	}
	verified, err := VerifyOutputContent(PurposeImage, normalized)
	if err != nil {
		t.Fatalf("VerifyOutputContent() error = %v", err)
	}
	if verified.ContentType != OutputContentTypeJPEG || verified.ByteSize != int64(len(normalized)) || verified.SHA256 != sha256Sum(normalized) {
		t.Fatalf("VerifyOutputContent() = %+v", verified)
	}
	if _, err := VerifyOutputContent(PurposeImage, input); err == nil {
		t.Fatal("image with source metadata was accepted")
	}
}

func TestVerifyOutputContentRejectsMalformedImageAndUnsupportedPurpose(t *testing.T) {
	for _, test := range []struct {
		name    string
		purpose Purpose
		data    []byte
	}{
		{name: "invalid JPEG", purpose: PurposeImage, data: []byte("not-a-jpeg")},
		{name: "unsupported purpose", purpose: Purpose("video"), data: testJPEG(t)},
		{name: "oversized JPEG", purpose: PurposeImage, data: bytes.Repeat([]byte{0}, int(MaxGenerationImageOutputBytes+1))},
		{name: "oversized dimensions", purpose: PurposeImage, data: jpegWithDimensions(5000, 5000)},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyOutputContent(test.purpose, test.data); err == nil {
				t.Fatal("VerifyOutputContent() unexpectedly accepted content")
			}
		})
	}
}

func TestVerifyOutputContentAcceptsStaticTexturedGLB(t *testing.T) {
	data := testStaticGLB(t, 1, nil)
	verified, err := VerifyOutputContent(PurposeModel, data)
	if err != nil {
		t.Fatalf("VerifyOutputContent() error = %v", err)
	}
	if verified.ContentType != OutputContentTypeGLB || verified.ByteSize != int64(len(data)) || verified.SHA256 != sha256Sum(data) {
		t.Fatalf("VerifyOutputContent() = %+v", verified)
	}
}

func TestVerifyOutputContentRejectsStaticGLBViolations(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "wrong declared length", data: func() []byte {
			data := testStaticGLB(t, 1, nil)
			binary.LittleEndian.PutUint32(data[8:12], uint32(len(data)-4))
			return data
		}()},
		{name: "external image", data: testStaticGLB(t, 1, func(doc map[string]any) {
			doc["images"] = []any{map[string]any{"uri": "https://example.invalid/image.png"}}
		})},
		{name: "unknown required extension", data: testStaticGLB(t, 1, func(doc map[string]any) {
			doc["extensionsRequired"] = []string{"EXT_unapproved"}
		})},
		{name: "animated model", data: testStaticGLB(t, 1, func(doc map[string]any) {
			doc["animations"] = []any{map[string]any{}}
		})},
		{name: "triangle budget", data: testStaticGLB(t, MaxGenerationModelTriangles+1, nil)},
		{name: "texture decode budget", data: testStaticGLBWithTexture(t, 1, 2048, 2048, func(doc map[string]any) {
			images := doc["images"].([]any)
			textures := doc["textures"].([]any)
			for index := 1; index < 6; index++ {
				images = append(images, map[string]any{"bufferView": 2, "mimeType": "image/png"})
				textures = append(textures, map[string]any{"source": index})
			}
			doc["images"], doc["textures"] = images, textures
		})},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := VerifyOutputContent(PurposeModel, test.data); err == nil {
				t.Fatal("VerifyOutputContent() unexpectedly accepted content")
			}
		})
	}
}

func TestVerifyOutputContentRejectsNonFiniteGLBPosition(t *testing.T) {
	data := testStaticGLB(t, 1, nil)
	jsonLength := int(binary.LittleEndian.Uint32(data[12:16]))
	binStart := 20 + jsonLength + 8
	binary.LittleEndian.PutUint32(data[binStart:binStart+4], math.Float32bits(float32(math.NaN())))
	if _, err := VerifyOutputContent(PurposeModel, data); err == nil {
		t.Fatal("GLB with a non-finite position was accepted")
	}
}

func testStaticGLB(t *testing.T, triangles int, mutate func(map[string]any)) []byte {
	return testStaticGLBWithTexture(t, triangles, 2, 2, mutate)
}

func testStaticGLBWithTexture(t *testing.T, triangles, textureWidth, textureHeight int, mutate func(map[string]any)) []byte {
	t.Helper()
	if triangles < 1 {
		t.Fatal("triangles must be positive")
	}
	texture := testPNG(t, textureWidth, textureHeight)
	indexBytes := make([]byte, triangles*3*2)
	for i := 0; i < triangles*3; i++ {
		binary.LittleEndian.PutUint16(indexBytes[i*2:i*2+2], uint16(i%3))
	}
	textureOffset := (36 + len(indexBytes) + 3) &^ 3
	buffer := make([]byte, textureOffset+len(texture))
	for i, vertex := range [][3]float32{{0, 0, 0}, {1, 0, 0}, {0, 1, 0}} {
		for axis, coordinate := range vertex {
			binary.LittleEndian.PutUint32(buffer[(i*3+axis)*4:(i*3+axis+1)*4], math.Float32bits(coordinate))
		}
	}
	copy(buffer[36:], indexBytes)
	copy(buffer[textureOffset:], texture)
	document := map[string]any{
		"asset":   map[string]any{"version": "2.0"},
		"scene":   0,
		"scenes":  []any{map[string]any{"nodes": []int{0}}},
		"nodes":   []any{map[string]any{"mesh": 0}},
		"buffers": []any{map[string]any{"byteLength": len(buffer)}},
		"bufferViews": []any{
			map[string]any{"buffer": 0, "byteOffset": 0, "byteLength": 36, "target": 34962},
			map[string]any{"buffer": 0, "byteOffset": 36, "byteLength": len(indexBytes), "target": 34963},
			map[string]any{"buffer": 0, "byteOffset": textureOffset, "byteLength": len(texture)},
		},
		"accessors": []any{
			map[string]any{"bufferView": 0, "componentType": 5126, "count": 3, "type": "VEC3"},
			map[string]any{"bufferView": 1, "componentType": 5123, "count": triangles * 3, "type": "SCALAR"},
		},
		"meshes":    []any{map[string]any{"primitives": []any{map[string]any{"attributes": map[string]int{"POSITION": 0}, "indices": 1, "material": 0, "mode": 4}}}},
		"materials": []any{map[string]any{"pbrMetallicRoughness": map[string]any{"baseColorTexture": map[string]any{"index": 0}}}},
		"textures":  []any{map[string]any{"source": 0}},
		"images":    []any{map[string]any{"bufferView": 2, "mimeType": "image/png"}},
	}
	if mutate != nil {
		mutate(document)
	}
	jsonData, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	for len(jsonData)%4 != 0 {
		jsonData = append(jsonData, ' ')
	}
	for len(buffer)%4 != 0 {
		buffer = append(buffer, 0)
	}
	total := 12 + 8 + len(jsonData) + 8 + len(buffer)
	glb := make([]byte, 12, total)
	binary.LittleEndian.PutUint32(glb[0:4], 0x46546c67)
	binary.LittleEndian.PutUint32(glb[4:8], 2)
	binary.LittleEndian.PutUint32(glb[8:12], uint32(total))
	glb = binary.LittleEndian.AppendUint32(glb, uint32(len(jsonData)))
	glb = binary.LittleEndian.AppendUint32(glb, 0x4e4f534a)
	glb = append(glb, jsonData...)
	glb = binary.LittleEndian.AppendUint32(glb, uint32(len(buffer)))
	glb = binary.LittleEndian.AppendUint32(glb, 0x004e4942)
	glb = append(glb, buffer...)
	return glb
}

func testPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 180, G: 80, B: 45, A: 255})
	var output bytes.Buffer
	if err := png.Encode(&output, canvas); err != nil {
		t.Fatal(err)
	}
	return output.Bytes()
}

func jpegWithDimensions(width, height int) []byte {
	data := []byte{0xff, 0xd8, 0xff, 0xc0, 0x00, 0x11, 0x08, 0, 0, 0, 0, 3,
		1, 0x11, 0, 2, 0x11, 0, 3, 0x11, 0}
	binary.BigEndian.PutUint16(data[7:9], uint16(height))
	binary.BigEndian.PutUint16(data[9:11], uint16(width))
	return append(data, 0xff, 0xd9)
}
