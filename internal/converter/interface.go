package converter

import "github.com/Deali-Axy/ebook-generator/internal/model"

type Converter interface {
	Build(book model.Book) error
}
