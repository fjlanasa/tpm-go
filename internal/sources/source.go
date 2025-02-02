package sources

import "google.golang.org/protobuf/proto"

type Source[T proto.Message] interface {
	Source() <-chan T
}
