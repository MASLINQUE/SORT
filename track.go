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
	// open output file
	// fo, err := os.Create("input.txt")
	// if err != nil {
	// 	panic(err)
	// }
	// // close fo on exit and check for its returned error
	// defer func() {
	// 	if err := fo.Close(); err != nil {
	// 		panic(err)
	// 	}
	// }()
	// // make a write buffer
	// w := bufio.NewWriter(fo)
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
			// reqdata := s.Text()
			// cam_id := gjson.Parse(reqdata).Get("camera.%did").Int()
			// image_width := gjson.Parse(reqdata).Get("info.width").Float()
			// image_height := gjson.Parse(reqdata).Get("info.height").Float()
			// log.Printf("w %v, h %v", image_width, image_height)
			// if sort, ok := sortCash[cam_id]; ok {
			// 	track1(reqdata, sort, image_width, image_height)
			// } else {
			// 	sort := NewSORT(*maxpred, *minupd, *ioutr)
			// 	sortCash[cam_id] = &sort
			// 	track1(reqdata, &sort, image_width, image_height)
			// }

			reqdata := s.Bytes()
			cam_id := gjson.ParseBytes(reqdata).Get("camera.%did").Int()
			image_width := gjson.ParseBytes(reqdata).Get("info.width").Float()
			image_height := gjson.ParseBytes(reqdata).Get("info.height").Float()
			// log.Printf("w %v, h %v", image_width, image_height)
			if sort, ok := sortCash[cam_id]; ok {
				track1(reqdata, sort, image_width, image_height, fo)
			} else {
				sort := NewSORT(*maxpred, *minupd, *ioutr)
				sortCash[cam_id] = &sort
				track1(reqdata, &sort, image_width, image_height, fo)
			}
		}

	}
}

func track1(reqdata []byte, sort *SORT, width float64, height float64, fo *os.File) {

	items := gjson.ParseBytes(reqdata).Get("items")
	track_items := []gjson.Result{}
	track_items = append(track_items, items.Array()...)
	// for _, item := range items.Array() {

	// 	track_items = append(track_items, item)

	// 	// bbox := []float64{}
	// 	// item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
	// 	// 	bbox = append(bbox, value.Num)
	// 	// 	return true
	// 	// })
	// 	// bbox = []float64{bbox[0], bbox[1], bbox[0] + bbox[2], bbox[1] + bbox[3]}

	// 	// bbox_prob := append(bbox, item.Get("prob").Num)
	// 	// track_bboxes = append(track_bboxes, bbox_prob)
	// }
	// itemzzzzz, _ := sort.Updatez(track_items)

	fo.WriteString("check before updatez")
	itemzzzzz, _ := sort.Updatez(track_items, width, height, fo)
	test := fmt.Sprintf("%s", itemzzzzz)
	reqdata_st, _ := sjson.SetRaw(string(reqdata), "items", test)

	// fo.WriteString(reqdata_st)
	fmt.Println(string(reqdata_st))
}

// func track(reqdata []byte, sort *SORT, width float64, height float64) {

// 	items := gjson.ParseBytes(reqdata).Get("items")
// 	track_bboxes := [][]float64{}
// 	for _, item := range items.Array() {
// 		bbox := []float64{}
// 		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
// 			bbox = append(bbox, value.Num)
// 			return true
// 		})
// 		bbox = []float64{bbox[0] * width, bbox[1] * height, (bbox[0] + bbox[2]) * width, (bbox[1] + bbox[3]) * height}
// 		log.Printf("bbox bf update %v", bbox)
// 		bbox_prob := append(bbox, item.Get("prob").Num)
// 		track_bboxes = append(track_bboxes, bbox_prob)
// 	}
// 	sort.Update(track_bboxes)
// 	item_template := items.Array()[0]
// 	// log.Printf("item_template %v", item_template.String())
// 	// var test string
// 	for ind := range sort.Trackers {
// 		item_updated, _ := sjson.Set(item_template.String(), "bbox", sort.Trackers[ind].LastBBox)
// 		// log.Printf("item_updated %v", item_updated)
// 		item_updated, _ = sjson.Set(item_updated, "%did", sort.Trackers[ind].ID)
// 		// test += item_updated
// 		// reqdata, _ = sjson.SetBytes(reqdata, "items.-1", []byte(item_updated))
// 	}
// 	// test1 := fmt.Sprint(test)
// 	// reqdata, _ = sjson.SetBytes(reqdata, "items", []byte(test))
// 	// for ind := range items.Array() {
// 	// 	log.Printf("bboxes %v", sort.Trackers[ind].LastBBox)
// 	// 	pathkey := fmt.Sprintf("items.%d.id", ind)
// 	// 	reqdata, _ = sjson.SetBytes(reqdata, pathkey, strconv.FormatInt(sort.Trackers[ind].ID, 10))
// 	// }

// 	fmt.Println(string(reqdata))

// }
