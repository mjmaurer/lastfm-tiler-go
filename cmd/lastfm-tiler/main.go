package main

import (
	"flag"
	"image/jpeg"
	"log"
	"os"
	"strings"

	lib "github.com/mjmaurer/lastfm-tiler-go"
)

func main() {
	lastfmUsers := flag.String("lastfm-users", "", "[required] Comma separated list of users for which to post top charts. Ex: 'rj,joesmith'")
	lastfmKey := flag.String("lastfm-api-key", "", "[required] API key granted by a Lastfm app.")
	lastfmChartPeriod := flag.String("lastfm-period", "7day", "Time period for charts. One of: overall|7day|1month|3month|6month|12month. (Default 7day)")
	gridSize := flag.Int("grid-size", 3, "Size of one side of the tiled grid. (Default 3)")
	imgSizePx := flag.Int("img-size-px", 174, "Size of one side of a square album cover. Upscaling does not improve resolution (Default 174)")

	flag.Parse()

	lgr := log.New(os.Stdout, "", log.LstdFlags)
	c := &lib.Config{
		LastfmAPIKey: *lastfmKey,
		LastfmPeriod: *lastfmChartPeriod,
		GridSize:     *gridSize,
		ImgSizePx:    *imgSizePx,
		Logger:       lgr,
	}

	fmIds := strings.Split(*lastfmUsers, ",")

	for _, u := range fmIds {
		img, err := lib.MakeTiledGrid(c, u)
		if err != nil {
			lgr.Println("Error for user ", u, " while making grid: ", err.Error())
		}
		f, err := os.Create(u + ".jpg")
		if err != nil {
			panic(err)
		}
		defer f.Close()
		err = jpeg.Encode(f, img, &jpeg.Options{Quality: 100})
		if err != nil {
			lgr.Println("Error for user ", u, " while writing image file: ", err.Error())
		}
	}
}
