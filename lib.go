package lib

import (
	"errors"
	"image"
	"image/color"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"

	"github.com/disintegration/imaging"
	"github.com/shkh/lastfm-go/lastfm"
)

// Config ... For MakeTiledGrid
type Config struct {
	LastfmAPIKey string      // [required]
	LastfmPeriod string      // overall|7day|1month|3month|6month|12month. Default 7day
	ImgSizePx    int         // One side of square album cover. Upscaling doesn't improve resolution. Default 174
	GridSize     int         // One side of tiled grid. Default 3
	Logger       *log.Logger // Default to DevNull
}

type orderedImage struct {
	img image.Image
	i   int
}

// Worker pool for performing tile operations (http + resize)
var tilePool chan int

// FMTILE_POOL_SIZE ... Set an environment variable with this name to
// an integer to indicate max parallelism for tiling operations (http request + resizing image)
// Default is 5.
const FMTILE_POOL_SIZE string = "FMTILE_POOL_SIZE"

func init() {
	i, err := strconv.Atoi(os.Getenv(FMTILE_POOL_SIZE))
	if err != nil {
		tilePool = make(chan int, 5)
	} else {
		tilePool = make(chan int, i)
	}
}

// MakeTiledGrid ... Returns an image.Image grid as specified by the Config.
// Will only fail in the case of Lastfm Api error. Otherwise, any tile specific
// errors will cause that tile to fill black (and be logged if enabled).
// See FMTILE_POOL_SIZE for concurrency control.
func MakeTiledGrid(c *Config, lastfmID string) (image.Image, error) {
	err := c.validate()
	if err != nil {
		return nil, err
	}
	lgr := c.Logger
	lastfmAPI := lastfm.New(c.LastfmAPIKey, "secret-not-needed")

	gridTiles := c.GridSize * c.GridSize
	sidePx := c.ImgSizePx * c.GridSize
	lgr.Println("size ", sidePx, " grid ", gridTiles)
	result, err := lastfmAPI.User.GetTopAlbums(
		lastfm.P{"user": lastfmID, "period": c.LastfmPeriod, "limit": gridTiles})
	if err != nil {
		return nil, err
	} else if len(result.Albums) < gridTiles {
		lgr.Println("Less than 9 top albums returned. Filling the remaining space with black.")
	}

	tiles := make(chan orderedImage)
	for i := 0; i < gridTiles; i++ {
		var url string
		if len(result.Albums) > i {
			album := result.Albums[i]
			url = chooseImg(album.Images)
		}
		go addTile(c, url, i, tiles)
	}
	grid := imaging.New(sidePx, sidePx, color.NRGBA{0, 0, 0, 0})
	for i := 0; i < gridTiles; i++ {
		tile := <-tiles
		row := tile.i / c.GridSize
		col := int(math.Mod(float64(tile.i), float64(c.GridSize)))
		grid = imaging.Paste(grid, tile.img, image.Pt(col*c.ImgSizePx, row*c.ImgSizePx))

	}

	return grid, nil
}

func addTile(c *Config, url string, i int, tiles chan<- orderedImage) {
	tilePool <- i
	defer func() {
		<-tilePool
	}()
	var img image.Image
	if url != "" {
		var err error
		img, err = loadImg(url)
		if err != nil {
			c.Logger.Println("Couldn't load image (", url, "). Falling back to black. Err: ", err.Error())
			img = imaging.New(c.ImgSizePx, c.ImgSizePx, color.NRGBA{0, 0, 0, 0})
		}
		img = imaging.Resize(img, c.ImgSizePx, c.ImgSizePx, imaging.Lanczos)
	} else {
		img = imaging.New(c.ImgSizePx, c.ImgSizePx, color.NRGBA{0, 0, 0, 0})
	}
	tiles <- orderedImage{
		i:   i,
		img: img,
	}
}

func (c *Config) validate() error {
	if c.LastfmAPIKey == "" {
		return errors.New("lastfmApiKey is required in Config")
	}
	if c.LastfmPeriod == "" {
		c.LastfmPeriod = "7day"
	}
	if c.GridSize <= 0 {
		c.GridSize = 3
	}
	if c.ImgSizePx <= 0 {
		c.ImgSizePx = 174
	}
	if c.Logger == nil {
		c.Logger = log.New(os.NewFile(0, os.DevNull), "", 0)
	}

	return nil
}

func chooseImg(imgs []struct {
	Size string `xml:"size,attr"`
	Url  string `xml:",chardata"`
}) string {
	for _, img := range imgs {
		if img.Size == "extralarge" {
			return img.Url
		}
	}
	for _, img := range imgs {
		if img.Size == "large" {
			return img.Url
		}
	}
	return imgs[0].Url
}

func loadImg(url string) (image.Image, error) {
	response, err := http.Get(url)
	if err != nil {
		return nil, errors.New("Couldn't load image: " + url + " with error: " + err.Error())
	}
	defer response.Body.Close()

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return nil, errors.New("Couldn't load image: " + url + " with error: " + err.Error())
	}
	return img, nil
}
