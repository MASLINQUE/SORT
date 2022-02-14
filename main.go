package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

var maxpred = flag.Int("maxpred", 3, "Max predicts without update")
// var minupd = flag.Int("minupd", 2, "Min updates use prediction")
var ioutr = flag.Float64("ioutr", 0.4, "IOU threshold")
var maxunm = flag.Int("maxunm", 3, "Max unmatches before delete")

var sortCash = map[int64]*SORT{}

func main() {
	flag.Parse()
	// logrus.SetLevel(logrus.DebugLevel)
	s := bufio.NewScanner(os.Stdin)
	bufsize := 10 << 20
	buf := make([]byte, bufsize)
	s.Buffer(buf, bufsize)
	for {
		if s.Scan() {
			reqdata := s.Bytes()
			cam_id := gjson.ParseBytes(reqdata).Get("camera.%did").Int()
			image_width := gjson.ParseBytes(reqdata).Get("info.width").Float()
			image_height := gjson.ParseBytes(reqdata).Get("info.height").Float()
			if sort, ok := sortCash[cam_id]; ok {
				track(reqdata, sort, image_width, image_height)
			} else {
				sort := NewSORT(*maxpred, *ioutr, *maxunm)
				sortCash[cam_id] = &sort
				track(reqdata, &sort, image_width, image_height)
			}
		}

	}
}

func track(reqdata []byte, sort *SORT, width float64, height float64) {

	items := gjson.ParseBytes(reqdata).Get("items")
	if len(items.Array()) == 0 {
		fmt.Println(string(reqdata))
		return
	}
	itemsArray := []gjson.Result{}
	itemsArray = append(itemsArray, items.Array()...)

	bboxesAndIDs, _ := sort.Update(itemsArray, width, height)
	responseStringArray := []string{}

	for _, item := range itemsArray {
		bbox_det := []float64{}
		item_str := item.String()
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox_det = append(bbox_det, value.Num)
			return true
		})
		bbox_det = []float64{bbox_det[0], bbox_det[1], bbox_det[0] + bbox_det[2], bbox_det[1] + bbox_det[3]}
		// bbox_det = []float64{bbox_det[0] * width, bbox_det[1] * height, bbox_det[0]*width + bbox_det[2]*width, bbox_det[1]*height + bbox_det[3]*height}
		for _, bbox_trc := range bboxesAndIDs {
			// iou := IOU(bbox_det, bbox_trc)
			// log.Printf("bbox_det %v, bbox_trc %v, iou -- %v", bbox_det, bbox_trc, mathBboxes(bbox_det, bbox_trc))
			if mathBboxes(bbox_det, bbox_trc) {
				item_str, _ = sjson.Set(item_str, "id", fmt.Sprintf("%.0f", bbox_trc[len(bbox_trc)-1]))
				responseStringArray = append(responseStringArray, item_str)
				break
			}
		}
	}
	responseString := fmt.Sprintf("[%s]", strings.Join(responseStringArray, ", "))

	reqdata_st, _ := sjson.SetRaw(string(reqdata), "items", responseString)
	fmt.Println(string(reqdata_st))
}
