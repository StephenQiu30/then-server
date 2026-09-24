package generation

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/jpeg"
	_ "image/png"
	"math"

	"github.com/qmuntal/gltf"
	"github.com/qmuntal/gltf/modeler"
)

const (
	MaxGenerationImagePixels    int64 = 24_000_000
	MaxGenerationTexturePixels  int64 = 24_000_000
	MaxGenerationTextureEdge          = 2048
	MaxGenerationModelTriangles       = 50_000
)

// VerifiedOutputContent contains facts calculated from decoded bytes rather
// than copied from provider or object-store metadata.
type VerifiedOutputContent struct {
	ContentType string
	ByteSize    int64
	SHA256      string
}

// NormalizeGenerationImage decodes and re-encodes a bounded JPEG so adapters
// can remove metadata before writing the private object version.
func NormalizeGenerationImage(data []byte) ([]byte, error) {
	if len(data) == 0 || int64(len(data)) > MaxGenerationImageOutputBytes || !hasJPEGStart(data) {
		return nil, ErrInvalidGenerationOutput
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || !validGenerationImageDimensions(config.Width, config.Height) {
		return nil, ErrInvalidGenerationOutput
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "jpeg" || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return nil, ErrInvalidGenerationOutput
	}
	var normalized bytes.Buffer
	if err := jpeg.Encode(&normalized, decoded, &jpeg.Options{Quality: 90}); err != nil || int64(normalized.Len()) > MaxGenerationImageOutputBytes {
		return nil, ErrInvalidGenerationOutput
	}
	return normalized.Bytes(), nil
}

// VerifyOutputContent validates bytes before a result worker publishes the
// private asset and recomputes its MIME type, length and SHA-256.
func VerifyOutputContent(purpose Purpose, data []byte) (VerifiedOutputContent, error) {
	contentType, limit := OutputContentTypeJPEG, MaxGenerationImageOutputBytes
	if purpose == PurposeModel {
		contentType, limit = OutputContentTypeGLB, MaxGenerationModelOutputBytes
	} else if purpose != PurposeImage {
		return VerifiedOutputContent{}, ErrInvalidGenerationOutput
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return VerifiedOutputContent{}, ErrInvalidGenerationOutput
	}
	var err error
	if purpose == PurposeImage {
		err = verifyGenerationImage(data)
	} else {
		err = verifyGenerationModel(data)
	}
	if err != nil {
		return VerifiedOutputContent{}, ErrInvalidGenerationOutput
	}
	digest := sha256Sum(data)
	return VerifiedOutputContent{ContentType: contentType, ByteSize: int64(len(data)), SHA256: digest}, nil
}

func verifyGenerationImage(data []byte) error {
	if !hasJPEGStart(data) || !sanitizedJPEGMarkers(data) {
		return ErrInvalidGenerationOutput
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || format != "jpeg" || !validGenerationImageDimensions(config.Width, config.Height) {
		return ErrInvalidGenerationOutput
	}
	decoded, format, err := image.Decode(bytes.NewReader(data))
	if err != nil || format != "jpeg" || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
		return ErrInvalidGenerationOutput
	}
	return nil
}

func validGenerationImageDimensions(width, height int) bool {
	return width > 0 && height > 0 && int64(width) <= MaxGenerationImagePixels && int64(height) <= MaxGenerationImagePixels && int64(width)*int64(height) <= MaxGenerationImagePixels
}

func hasJPEGStart(data []byte) bool {
	return len(data) >= 4 && data[0] == 0xff && data[1] == 0xd8
}

// sanitizedJPEGMarkers permits the JFIF header emitted by Go's encoder and
// rejects application/comment segments that can carry source metadata.
func sanitizedJPEGMarkers(data []byte) bool {
	if !hasJPEGStart(data) {
		return false
	}
	i := 2
	inScan := false
	for i < len(data) {
		if inScan {
			markerStart := bytes.IndexByte(data[i:], 0xff)
			if markerStart < 0 {
				return false
			}
			i += markerStart
			markerEnd := i + 1
			for markerEnd < len(data) && data[markerEnd] == 0xff {
				markerEnd++
			}
			if markerEnd >= len(data) {
				return false
			}
			marker := data[markerEnd]
			if marker == 0x00 || marker >= 0xd0 && marker <= 0xd7 {
				i = markerEnd + 1
				continue
			}
			inScan = false
		}
		if i >= len(data) || data[i] != 0xff {
			return false
		}
		for i < len(data) && data[i] == 0xff {
			i++
		}
		if i >= len(data) {
			return false
		}
		marker := data[i]
		i++
		if marker == 0xd9 {
			return i == len(data)
		}
		if marker == 0x00 || marker == 0xd8 || marker == 0x01 || marker >= 0xd0 && marker <= 0xd7 {
			return false
		}
		if i+2 > len(data) {
			return false
		}
		segmentLength := int(binary.BigEndian.Uint16(data[i : i+2]))
		if segmentLength < 2 || segmentLength > len(data)-i {
			return false
		}
		segmentEnd := i + segmentLength
		payload := data[i+2 : segmentEnd]
		switch {
		case marker == 0xe0:
			if len(payload) != 14 || !bytes.HasPrefix(payload, []byte("JFIF\x00")) {
				return false
			}
		case marker >= 0xe1 && marker <= 0xef, marker == 0xfe:
			return false
		}
		i = segmentEnd
		if marker == 0xda {
			inScan = true
		}
	}
	return false
}

func verifyGenerationModel(data []byte) error {
	jsonChunk, err := validateGLBEnvelope(data)
	if err != nil {
		return err
	}
	var root map[string]json.RawMessage
	if err := json.Unmarshal(jsonChunk, &root); err != nil || root == nil {
		return ErrInvalidGenerationOutput
	}
	doc := new(gltf.Document)
	if err := gltf.NewDecoder(bytes.NewReader(data)).Decode(doc); err != nil {
		return err
	}
	if doc.Asset.Version != "2.0" || doc.Asset.MinVersion != "" && doc.Asset.MinVersion != "2.0" ||
		len(doc.Buffers) != 1 || doc.Buffers[0] == nil || doc.Buffers[0].URI != "" || len(doc.Buffers[0].Data) == 0 ||
		len(doc.Animations) != 0 || len(doc.Skins) != 0 || len(doc.ExtensionsRequired) != 0 ||
		len(doc.Meshes) == 0 || len(doc.Images) == 0 || len(doc.Textures) == 0 || len(doc.Scenes) == 0 || doc.Scene == nil {
		return ErrInvalidGenerationOutput
	}
	if err := validateModelBufferViews(doc); err != nil {
		return err
	}
	if err := validateModelTextures(doc); err != nil {
		return err
	}
	if err := validateModelGeometry(doc); err != nil {
		return err
	}
	return validateModelScene(doc)
}

func validateGLBEnvelope(data []byte) ([]byte, error) {
	if len(data) < 28 || binary.LittleEndian.Uint32(data[0:4]) != 0x46546c67 || binary.LittleEndian.Uint32(data[4:8]) != 2 || uint64(binary.LittleEndian.Uint32(data[8:12])) != uint64(len(data)) {
		return nil, ErrInvalidGenerationOutput
	}
	jsonLength := int(binary.LittleEndian.Uint32(data[12:16]))
	if binary.LittleEndian.Uint32(data[16:20]) != 0x4e4f534a || jsonLength == 0 || jsonLength%4 != 0 || jsonLength > len(data)-20 {
		return nil, ErrInvalidGenerationOutput
	}
	jsonEnd := 20 + jsonLength
	if len(data)-jsonEnd < 8 || binary.LittleEndian.Uint32(data[jsonEnd+4:jsonEnd+8]) != 0x004e4942 {
		return nil, ErrInvalidGenerationOutput
	}
	binLength := int(binary.LittleEndian.Uint32(data[jsonEnd : jsonEnd+4]))
	if binLength == 0 || binLength%4 != 0 || binLength != len(data)-jsonEnd-8 {
		return nil, ErrInvalidGenerationOutput
	}
	return data[20:jsonEnd], nil
}

func validateModelBufferViews(doc *gltf.Document) error {
	buffer := doc.Buffers[0]
	if buffer.ByteLength <= 0 || len(buffer.Data) != buffer.ByteLength {
		return ErrInvalidGenerationOutput
	}
	for _, view := range doc.BufferViews {
		if view == nil || view.Buffer != 0 || view.ByteOffset < 0 || view.ByteLength <= 0 || view.ByteOffset > buffer.ByteLength || view.ByteLength > buffer.ByteLength-view.ByteOffset {
			return ErrInvalidGenerationOutput
		}
		if view.ByteStride != 0 && (view.ByteStride < 4 || view.ByteStride > 252 || view.ByteStride%4 != 0) {
			return ErrInvalidGenerationOutput
		}
	}
	return nil
}

func validateModelTextures(doc *gltf.Document) error {
	var decodedTexturePixels int64
	for _, imageAsset := range doc.Images {
		if imageAsset == nil || imageAsset.URI != "" || imageAsset.BufferView == nil || imageAsset.MimeType != "image/jpeg" && imageAsset.MimeType != "image/png" {
			return ErrInvalidGenerationOutput
		}
		if *imageAsset.BufferView < 0 || *imageAsset.BufferView >= len(doc.BufferViews) {
			return ErrInvalidGenerationOutput
		}
		data, err := modeler.ReadBufferView(doc, doc.BufferViews[*imageAsset.BufferView])
		if err != nil {
			return err
		}
		config, format, err := image.DecodeConfig(bytes.NewReader(data))
		if err != nil || format != expectedImageFormat(imageAsset.MimeType) || config.Width < 1 || config.Height < 1 || config.Width > MaxGenerationTextureEdge || config.Height > MaxGenerationTextureEdge {
			return ErrInvalidGenerationOutput
		}
		pixels := int64(config.Width) * int64(config.Height)
		if pixels > MaxGenerationTexturePixels-decodedTexturePixels {
			return ErrInvalidGenerationOutput
		}
		decodedTexturePixels += pixels
		decoded, format, err := image.Decode(bytes.NewReader(data))
		if err != nil || format != expectedImageFormat(imageAsset.MimeType) || decoded.Bounds().Dx() != config.Width || decoded.Bounds().Dy() != config.Height {
			return ErrInvalidGenerationOutput
		}
	}
	for _, texture := range doc.Textures {
		if texture == nil || texture.Source == nil || *texture.Source < 0 || *texture.Source >= len(doc.Images) || texture.Sampler != nil && (*texture.Sampler < 0 || *texture.Sampler >= len(doc.Samplers)) {
			return ErrInvalidGenerationOutput
		}
	}
	used := false
	checkTextureIndex := func(index *int) bool {
		if index == nil {
			return true
		}
		if *index < 0 || *index >= len(doc.Textures) {
			return false
		}
		used = true
		return true
	}
	for _, material := range doc.Materials {
		if material == nil {
			return ErrInvalidGenerationOutput
		}
		if material.PBRMetallicRoughness != nil {
			for _, info := range []*gltf.TextureInfo{material.PBRMetallicRoughness.BaseColorTexture, material.PBRMetallicRoughness.MetallicRoughnessTexture} {
				if !validModelTextureInfo(info, len(doc.Textures)) {
					return ErrInvalidGenerationOutput
				}
				used = used || info != nil
			}
		}
		if material.EmissiveTexture != nil && !checkTextureIndex(&material.EmissiveTexture.Index) {
			return ErrInvalidGenerationOutput
		}
		if material.NormalTexture != nil && !checkTextureIndex(material.NormalTexture.Index) {
			return ErrInvalidGenerationOutput
		}
		if material.OcclusionTexture != nil && !checkTextureIndex(material.OcclusionTexture.Index) {
			return ErrInvalidGenerationOutput
		}
	}
	if !used {
		return ErrInvalidGenerationOutput
	}
	return nil
}

func validModelTextureInfo(info *gltf.TextureInfo, textureCount int) bool {
	return info == nil || info.Index >= 0 && info.Index < textureCount
}

func expectedImageFormat(mimeType string) string {
	if mimeType == "image/jpeg" {
		return "jpeg"
	}
	return "png"
}

func validateModelGeometry(doc *gltf.Document) error {
	var triangles int64
	for _, mesh := range doc.Meshes {
		if mesh == nil || len(mesh.Primitives) == 0 {
			return ErrInvalidGenerationOutput
		}
		for _, primitive := range mesh.Primitives {
			if primitive == nil || primitive.Mode != gltf.PrimitiveTriangles || len(primitive.Targets) != 0 || primitive.Material == nil || *primitive.Material < 0 || *primitive.Material >= len(doc.Materials) {
				return ErrInvalidGenerationOutput
			}
			positionIndex, exists := primitive.Attributes[gltf.POSITION]
			if !exists || positionIndex < 0 || positionIndex >= len(doc.Accessors) {
				return ErrInvalidGenerationOutput
			}
			positionAccessor := doc.Accessors[positionIndex]
			if positionAccessor == nil || positionAccessor.Count < 1 || int64(positionAccessor.Count) > MaxGenerationModelOutputBytes/12 {
				return ErrInvalidGenerationOutput
			}
			positions, err := modeler.ReadPosition(doc, positionAccessor, nil)
			if err != nil || len(positions) != positionAccessor.Count {
				return ErrInvalidGenerationOutput
			}
			for _, position := range positions {
				for _, coordinate := range position {
					if math.IsNaN(float64(coordinate)) || math.IsInf(float64(coordinate), 0) {
						return ErrInvalidGenerationOutput
					}
				}
			}
			primitiveCount := len(positions)
			if primitive.Indices != nil {
				if *primitive.Indices < 0 || *primitive.Indices >= len(doc.Accessors) {
					return ErrInvalidGenerationOutput
				}
				indicesAccessor := doc.Accessors[*primitive.Indices]
				if indicesAccessor == nil || indicesAccessor.Count < 1 || indicesAccessor.Count > MaxGenerationModelTriangles*3 {
					return ErrInvalidGenerationOutput
				}
				indices, err := modeler.ReadIndices(doc, indicesAccessor, nil)
				if err != nil || len(indices) != indicesAccessor.Count {
					return ErrInvalidGenerationOutput
				}
				for _, index := range indices {
					if uint64(index) >= uint64(len(positions)) {
						return ErrInvalidGenerationOutput
					}
				}
				primitiveCount = len(indices)
			}
			if primitiveCount%3 != 0 {
				return ErrInvalidGenerationOutput
			}
			triangles += int64(primitiveCount / 3)
			if triangles > MaxGenerationModelTriangles {
				return ErrInvalidGenerationOutput
			}
		}
	}
	return nil
}

func validateModelScene(doc *gltf.Document) error {
	if *doc.Scene < 0 || *doc.Scene >= len(doc.Scenes) || doc.Scenes[*doc.Scene] == nil || len(doc.Scenes[*doc.Scene].Nodes) == 0 {
		return ErrInvalidGenerationOutput
	}
	for _, node := range doc.Nodes {
		if node == nil || node.Skin != nil || node.Mesh != nil && (*node.Mesh < 0 || *node.Mesh >= len(doc.Meshes)) {
			return ErrInvalidGenerationOutput
		}
		for _, child := range node.Children {
			if child < 0 || child >= len(doc.Nodes) {
				return ErrInvalidGenerationOutput
			}
		}
	}
	for _, root := range doc.Scenes[*doc.Scene].Nodes {
		if root < 0 || root >= len(doc.Nodes) {
			return ErrInvalidGenerationOutput
		}
	}
	state := make([]uint8, len(doc.Nodes))
	visibleMesh := false
	var visit func(int) error
	visit = func(index int) error {
		if state[index] == 1 {
			return ErrInvalidGenerationOutput
		}
		if state[index] == 2 {
			return nil
		}
		state[index] = 1
		node := doc.Nodes[index]
		if node.Mesh != nil {
			visibleMesh = true
		}
		for _, child := range node.Children {
			if err := visit(child); err != nil {
				return err
			}
		}
		state[index] = 2
		return nil
	}
	for _, root := range doc.Scenes[*doc.Scene].Nodes {
		if err := visit(root); err != nil {
			return err
		}
	}
	if !visibleMesh {
		return ErrInvalidGenerationOutput
	}
	return nil
}

func sha256Sum(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}
