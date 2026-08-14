package ogimage

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/SouichiroTsujimoto/unagi/internal/braille"
	"github.com/SouichiroTsujimoto/unagi/internal/feature/article"
	"github.com/go-opentype/fonts/notoemoji"
	"github.com/go-opentype/fonts/notosansjp"
	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/fixed"
)

const (
	Width           = 1200
	Height          = 630
	RendererVersion = 7
	renderScale     = 2
)

var (
	darkBackground = color.RGBA{A: 255}
	ink            = color.RGBA{R: 28, G: 27, B: 24, A: 255}
	lightInk       = color.RGBA{R: 255, G: 253, B: 242, A: 255}
	darkMuted      = color.RGBA{R: 201, G: 196, B: 181, A: 255}
	accent         = color.RGBA{R: 255, G: 216, B: 45, A: 255}
	darkLine       = color.RGBA{R: 74, G: 71, B: 63, A: 255}
)

// Images renders deterministic article social cards.
type Images struct {
	font      *opentype.Font
	emojiFont *opentype.Font
}

func New() (*Images, error) {
	f, err := opentype.Parse(notosansjp.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse OGP font: %w", err)
	}
	emojiFont, err := opentype.Parse(notoemoji.TTF)
	if err != nil {
		return nil, fmt.Errorf("parse OGP emoji font: %w", err)
	}
	return &Images{font: f, emojiFont: emojiFont}, nil
}

// Path returns the cache-versioned public image path for an article.
func Path(item article.Article) string {
	name := fmt.Sprintf("%d-%d-%d.png", item.RevisionID, item.OGVersion, RendererVersion)
	return path.Join("/og/articles", item.Slug, name)
}

func Alt(item article.Article) string {
	return item.Title + "のOGP画像"
}

func (images *Images) face(size float64) (font.Face, error) {
	return opentype.NewFace(images.font, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

func (images *Images) emojiFace(size float64) (font.Face, error) {
	return opentype.NewFace(images.emojiFont, &opentype.FaceOptions{
		Size:    size,
		DPI:     72,
		Hinting: font.HintingFull,
	})
}

// RenderPNG renders the article's selected design as a 1200×630 PNG.
func (images *Images) RenderPNG(item article.Article) ([]byte, error) {
	template := item.OGTemplate
	if template == "" {
		template = article.DefaultOGTemplate
	}

	bg, textColor, secondaryColor, ruleColor, dotColor, emojiColor := darkBackground, lightInk, darkMuted, darkLine, accent, accent
	if template == article.OGTemplateEditorial {
		bg, textColor, secondaryColor, ruleColor, dotColor, emojiColor = accent, ink, ink, ink, ink, ink
	}

	canvas := image.NewRGBA(image.Rect(0, 0, scaled(Width), scaled(Height)))
	draw.Draw(canvas, canvas.Bounds(), &image.Uniform{C: bg}, image.Point{}, draw.Src)
	if template == article.OGTemplateDotDark {
		draw.Draw(canvas, image.Rect(0, 0, scaled(12), scaled(Height)), &image.Uniform{C: accent}, image.Point{}, draw.Src)
	}
	draw.Draw(canvas, image.Rect(scaled(76), scaled(82), scaled(1124), scaled(84)), &image.Uniform{C: ruleColor}, image.Point{}, draw.Src)

	labelFace, err := images.face(24 * renderScale)
	if err != nil {
		return nil, err
	}
	defer labelFace.Close()
	drawTextHeavy(canvas, labelFace, textColor, scaled(78), scaled(61), "unagi", renderScale)
	drawTextRightHeavy(canvas, labelFace, secondaryColor, scaled(1122), scaled(61), "wuhu1sland", renderScale)

	if item.Emoji != "" {
		emojiFace, err := images.emojiFace(58 * renderScale)
		if err != nil {
			return nil, err
		}
		defer emojiFace.Close()
		drawText(canvas, emojiFace, emojiColor, scaled(78), scaled(176), item.Emoji)
	}
	drawBraille(canvas, scaled(874), scaled(226), braille.NoiseGrid(item.Slug, 5, 5), dotColor, renderScale)

	titleFace, lines, err := images.fitTitle(item.Title, scaled(760), renderScale)
	if err != nil {
		return nil, err
	}
	defer titleFace.Close()
	lineHeight := titleFace.Metrics().Height.Ceil() - scaled(4)
	titleY := scaled(270)
	for _, text := range lines {
		drawTextHeavy(canvas, titleFace, textColor, scaled(78), titleY, text, renderScale)
		titleY += lineHeight
	}

	topicFace, err := images.face(20 * renderScale)
	if err != nil {
		return nil, err
	}
	defer topicFace.Close()
	drawTopics(canvas, topicFace, secondaryColor, item.Topics, renderScale)

	output := image.NewRGBA(image.Rect(0, 0, Width, Height))
	xdraw.CatmullRom.Scale(output, output.Bounds(), canvas, canvas.Bounds(), draw.Src, nil)

	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := encoder.Encode(&out, output); err != nil {
		return nil, fmt.Errorf("encode OGP PNG: %w", err)
	}
	return out.Bytes(), nil
}

func (images *Images) fitTitle(title string, maxWidth, scale int) (font.Face, []string, error) {
	title = strings.TrimSpace(title)
	if title == "" {
		title = "Untitled"
	}
	for _, size := range []float64{68, 62, 56, 50, 44} {
		face, err := images.face(size * float64(scale))
		if err != nil {
			return nil, nil, err
		}
		lines := wrap(face, title, maxWidth)
		if len(lines) <= 3 {
			return face, lines, nil
		}
		face.Close()
	}
	face, err := images.face(44 * float64(scale))
	if err != nil {
		return nil, nil, err
	}
	lines := wrap(face, title, maxWidth)
	if len(lines) > 3 {
		lines = lines[:3]
		lines[2] = truncate(face, lines[2], maxWidth)
	}
	return face, lines, nil
}

func wrap(face font.Face, text string, maxWidth int) []string {
	var lines []string
	var current strings.Builder
	for _, token := range wrapTokens(text) {
		if strings.TrimSpace(token) == "" {
			if current.Len() > 0 {
				current.WriteString(token)
			}
			continue
		}
		candidate := current.String() + token
		if current.Len() > 0 && font.MeasureString(face, candidate).Ceil() > maxWidth {
			lines = append(lines, strings.TrimSpace(current.String()))
			current.Reset()
		}
		if font.MeasureString(face, token).Ceil() <= maxWidth {
			current.WriteString(token)
			continue
		}
		for _, r := range token {
			candidate = current.String() + string(r)
			if current.Len() > 0 && font.MeasureString(face, candidate).Ceil() > maxWidth {
				lines = append(lines, strings.TrimSpace(current.String()))
				current.Reset()
			}
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		lines = append(lines, strings.TrimSpace(current.String()))
	}
	return lines
}

func wrapTokens(text string) []string {
	var tokens []string
	var current strings.Builder
	var currentKind byte
	flush := func() {
		if current.Len() == 0 {
			return
		}
		tokens = append(tokens, current.String())
		current.Reset()
	}
	for _, r := range text {
		var kind byte
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			kind = 's'
		case r >= 0x21 && r <= 0x7e:
			kind = 'a'
		default:
			kind = 'r'
		}
		if kind == 'r' {
			flush()
			tokens = append(tokens, string(r))
			currentKind = 0
			continue
		}
		if currentKind != 0 && currentKind != kind {
			flush()
		}
		currentKind = kind
		current.WriteRune(r)
	}
	flush()
	return tokens
}

func truncate(face font.Face, text string, maxWidth int) string {
	const suffix = "…"
	for utf8.RuneCountInString(text) > 0 && font.MeasureString(face, text+suffix).Ceil() > maxWidth {
		_, size := utf8.DecodeLastRuneInString(text)
		text = text[:len(text)-size]
	}
	return strings.TrimSpace(text) + suffix
}

func scaled(value int) int {
	return value * renderScale
}

func drawText(dst draw.Image, face font.Face, c color.Color, x, baseline int, text string) {
	d := font.Drawer{
		Dst:  dst,
		Src:  image.NewUniform(c),
		Face: face,
		Dot:  fixed.P(x, baseline),
	}
	d.DrawString(text)
}

func drawTextHeavy(dst draw.Image, face font.Face, c color.Color, x, baseline int, text string, scale int) {
	drawText(dst, face, c, x, baseline, text)
	drawText(dst, face, c, x+scale, baseline, text)
	drawText(dst, face, c, x+2*scale, baseline, text)
	drawText(dst, face, c, x, baseline+scale, text)
	drawText(dst, face, c, x+scale, baseline+scale, text)
}

func drawTextRightHeavy(dst draw.Image, face font.Face, c color.Color, right, baseline int, text string, scale int) {
	width := font.MeasureString(face, text).Ceil()
	drawTextHeavy(dst, face, c, right-width-2*scale, baseline, text, scale)
}

func drawBraille(dst draw.Image, originX, originY int, pattern braille.DotGrid, dotColor color.Color, scale int) {
	const (
		step   = 58
		radius = 19
	)
	for i, on := range pattern.On {
		if !on {
			continue
		}
		cx := originX + (i%5)*step*scale
		cy := originY + (i/5)*step*scale
		fillCircle(dst, cx, cy, radius*scale, dotColor)
	}
}

func fillCircle(dst draw.Image, cx, cy, radius int, c color.Color) {
	for y := -radius; y <= radius; y++ {
		for x := -radius; x <= radius; x++ {
			if x*x+y*y <= radius*radius {
				dst.Set(cx+x, cy+y, c)
			}
		}
	}
}

func drawTopics(dst draw.Image, face font.Face, textColor color.Color, topics []string, scale int) {
	x := 78 * scale
	y := 548 * scale
	for i, topic := range topics {
		if i == 3 {
			break
		}
		label := "#" + topic
		width := font.MeasureString(face, label).Ceil() + 30*scale
		drawTextHeavy(dst, face, textColor, x+15*scale, y, label, scale)
		x += width + 12*scale
	}
}
