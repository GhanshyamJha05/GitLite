package object

// Blob represents raw file data stored in the object database.
// Blobs are unaware of file names, timestamps, or file system permissions.
type Blob struct {
	Data []byte
}

// NewBlob creates a new Blob instance with the provided byte slice.
func NewBlob(data []byte) *Blob {
	return &Blob{
		Data: data,
	}
}

// Type returns TypeBlob.
func (b *Blob) Type() Type {
	return TypeBlob
}

// SerializeContent returns the underlying file payload.
func (b *Blob) SerializeContent() []byte {
	return b.Data
}
