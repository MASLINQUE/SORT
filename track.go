package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var maxpred = flag.Int("maxpred", 3, "Max predicts without update")
var minupd = flag.Int("minupd", 2, "Min updates use prediction")
var ioutr = flag.Float64("ioutr", 0.4, "IOU threshold")

var sortCash = map[int64]*SORT{}

func main() {
	// logrus.SetLevel(logrus.DebugLevel)
	s := bufio.NewScanner(os.Stdin)
	bufsize := 10 << 20
	buf := make([]byte, bufsize)
	s.Buffer(buf, bufsize)
	fo, err := os.Create("debug.txt")
	if err != nil {
		panic(err)
	}
	// close fo on exit and check for its returned error
	defer func() {
		if err := fo.Close(); err != nil {
			panic(err)
		}
	}()
	for {
		if s.Scan() {
			reqdata := s.Bytes()
			cam_id := gjson.ParseBytes(reqdata).Get("camera.%did").Int()
			image_width := gjson.ParseBytes(reqdata).Get("info.width").Float()
			image_height := gjson.ParseBytes(reqdata).Get("info.height").Float()
			// log.Printf("w %v, h %v", image_width, image_height)
			if sort, ok := sortCash[cam_id]; ok {
				track(reqdata, sort, image_width, image_height, fo)
			} else {
				sort := NewSORT(*maxpred, *minupd, *ioutr)
				sortCash[cam_id] = &sort
				track(reqdata, &sort, image_width, image_height, fo)
			}
		}

	}
}

func track(reqdata []byte, sort *SORT, width float64, height float64, fo *os.File) {

	items := gjson.ParseBytes(reqdata).Get("items")
	track_items := []gjson.Result{}
	track_items = append(track_items, items.Array()...)

	fo.WriteString("check before updatez")
	itemzzzzz, _ := sort.Updatez(track_items, width, height, fo)
	test := fmt.Sprintf("%s", itemzzzzz)
	reqdata_st, _ := sjson.SetRaw(string(reqdata), "items", test)

	// fo.WriteString(reqdata_st)
	fmt.Println(string(reqdata_st))
}
