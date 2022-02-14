package main

import (
	"fmt"
	"math"

	"github.com/cpmech/gosl/graph"
	"github.com/tidwall/gjson"
)

//SORT Detection tracking
type SORT struct {
	maxPredictsWithoutUpdate int
	minUpdatesUsePrediction  int
	maxUnmatches             int
	iouThreshold             float64
	Trackers                 []*KalmanBoxTracker
	FrameCount               int
}

//NewSORT initializes a new SORT tracking session
func NewSORT(maxPredictsWithoutUpdate int, iouThreshold float64, maxunm int) SORT {
	return SORT{
		maxPredictsWithoutUpdate: maxPredictsWithoutUpdate,
		minUpdatesUsePrediction:  0,
		iouThreshold:             iouThreshold,
		maxUnmatches:             maxunm,
		Trackers:                 make([]*KalmanBoxTracker, 0),
		FrameCount:               0,
	}
}

//Update update trackers from detections
//     Params:
//       dets - a numpy array of detections in the format [[x1,y1,x2,y2,score],[x1,y1,x2,y2,score],...]
//     Requires: this method must be called once for each frame even with empty detections.
//     Returns the a similar array, where the last column is the object ID.
//     NOTE: The number of objects returned may differ from the number of detections provided.

func (s *SORT) RemoveTrackByIndexs(indxs []int) {
	var resultVec = []*KalmanBoxTracker{}

	for _, indx := range indxs {
		s.Trackers[indx] = nil
	}
	for _, val := range s.Trackers {
		if val != nil {
			resultVec = append(resultVec, val)
		}
	}
	s.Trackers = resultVec

}

func (s *SORT) Update(items []gjson.Result, width float64, height float64) ([][]float64, [][]float64) {

	dets := [][]float64{}
	for _, item := range items {
		bbox := []float64{}
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox = append(bbox, value.Num)
			return true
		})
		// bbox = []float64{bbox[0] * width, bbox[1] * height, bbox[0]*width + bbox[2]*width, bbox[1]*height + bbox[3]*height, item.Get("prob").Num}
		bbox = []float64{bbox[0], bbox[1], bbox[0] + bbox[2], bbox[1] + bbox[3], item.Get("prob").Num}

		dets = append(dets, bbox)

	}
	bboxesAndID := [][]float64{}

	s.FrameCount = s.FrameCount + 1

	matched, unmatchedDets, unmatchedTrks := associateDetectionsToTrackers(dets, s.Trackers, s.iouThreshold, s.minUpdatesUsePrediction)

	// log.Printf("matched %v", matched)
	// log.Printf("unmatchedDets %v", unmatchedDets)
	// log.Printf("unmatchedTrks %v", unmatchedTrks)

	// update matched trackers with assigned detections
	for t := 0; t < len(s.Trackers); t++ {
		tracker := s.Trackers[t]
		//is this tracker still matched?
		if !contains(unmatchedTrks, t) {
			for _, det := range matched {
				if det[1] == t {
					bbox := dets[det[0]]
					bboxID := bbox[:len(bbox)-1]

					tracker.Update(bboxID)
					bboxID = append(bboxID, float64(tracker.ID))
					bboxesAndID = append(bboxesAndID, bboxID)
					break
				}
			}

		}
	}

	// create and initialise new trackers for unmatched detections
	for _, udet := range unmatchedDets {
		det := dets[udet]
		detnonScore := det[:len(det)-1]
		// fmt.Printf("\n\n dets %v \n\n", detnonScore)
		trk, _ := NewKalmanBoxTracker(detnonScore)
		ls_bbox := trk.LastBBox
		ls_bbox = append(ls_bbox, float64(trk.ID))
		// fmt.Printf("\n\n ls_bbox %v \n\n", ls_bbox)
		s.Trackers = append(s.Trackers, &trk)

		bboxesAndID = append(bboxesAndID, ls_bbox)

	}

	delete_ind := []int{}
	for _, t := range unmatchedTrks {
		trk := s.Trackers[t]
		trk.Unmatches += 1

		if trk.PredictsSinceUpdate > s.maxPredictsWithoutUpdate || trk.Unmatches >= s.maxUnmatches {
			delete_ind = append(delete_ind, t)
		}
	}

	if len(delete_ind) > 0 {
		s.RemoveTrackByIndexs(delete_ind)
	}
	ct := ""
	for _, v := range s.Trackers {
		ct = ct + fmt.Sprintf("[id=%d bbox=%v updates=%d] ", v.ID, v.LastBBox, v.Updates)
	}

	return bboxesAndID, dets
}

func contains(list []int, value int) bool {
	found := false
	for _, v := range list {
		if v == value {
			found = true
			break
		}
	}
	return found
}

//   Assigns detections to tracked object (both represented as bounding boxes)
//   Returns 3 lists of indexes: matches, unmatched_detections and unmatched_trackers
func associateDetectionsToTrackers(detections [][]float64, trackers []*KalmanBoxTracker, iouThreshold float64, minUpdatesUsePrediction int) ([][]int, []int, []int) {
	if len(trackers) == 0 {
		det := make([]int, 0)
		for i := range detections {
			det = append(det, i)
		}
		return [][]int{}, det, []int{}
	}

	ld := len(detections)
	lt := len(trackers)

	if ld == 0 {
		unmatchedTrackers := make([]int, 0)
		for t := 0; t < lt; t++ {
			unmatchedTrackers = append(unmatchedTrackers, t)
			trackers[t].PredictNext()
		}
		// fmt.Printf(">>>>EMPTY DETECTIONS %d %d", ld, lt)
		return [][]int{}, []int{}, unmatchedTrackers
	}

	mk := graph.Munkres{}
	mk.Init(int(ld), int(lt))

	ious := make([][]float64, ld)
	for i := 0; i < len(ious); i++ {
		ious[i] = make([]float64, lt)
	}

	predicted := make([]bool, lt)
	for d := 0; d < ld; d++ {
		for t := 0; t < lt; t++ {
			trk := trackers[t]

			lastBbox := trk.LastBBox
			predictedBbox := trk.LastBBox

			if !predicted[t] {
				predictedBbox = trk.PredictNext()
				predicted[t] = true
				trk.PredictsSinceUpdate += 1
			} else {
				predictedBbox = trk.CurrentPrediction()
			}

			lIOU := IOU(detections[d], lastBbox)
			pIOu := IOU(detections[d], predictedBbox)

			v := math.Max(lIOU, pIOu)

			if lIOU > pIOu {
				trk.LastBBoxIOU = lastBbox
			} else {
				trk.LastBBoxIOU = predictedBbox
			}

			ious[d][t] = 1 - v
			//use simple last bbox if not enough updates in this tracker

			// tbbox := trk.LastBBox

			// //use prediction
			// if trk.Updates >= minUpdatesUsePrediction {
			// 	//in this frame request, predict just once
			// 	if !predicted[t] {
			// 		tbbox = trk.PredictNext()
			// 		predicted[t] = true
			// 	} else {
			// 		tbbox = trk.CurrentPrediction()
			// 	}
			// } else {
			// 	trk.SkipPredicts = trk.SkipPredicts + 1
			// }

			// v := IOU(detections[d], tbbox)
			// log.Printf("detection %v, [trackerID %v predict next %v], iou %v", detections[d], trk.ID, tbbox, v)
			// trk.LastBBoxIOU = tbbox

			//invert cost matrix (we want max cost here)
			// ious[d][t] = 1 - v
		}
	}
	// log.Printf("ious matrix %v", ious)

	//calculate best DETECTION vs TRACKER matches according to COST matrix
	mk.SetCostMatrix(ious)
	mk.Run()
	matchedIndices := [][]int{}
	for i, j := range mk.Links {
		if j != -1 {
			matchedIndices = append(matchedIndices, []int{i, j})
		}
	}

	unmatchedDetections := make([]int, 0)
	for d := 0; d < ld; d++ {
		found := false
		for _, v := range matchedIndices {
			if d == v[0] {
				found = true
				break
			}
		}
		if !found {

			unmatchedDetections = append(unmatchedDetections, d)
		}
	}

	unmatchedTrackers := make([]int, 0)
	for t := 0; t < lt; t++ {
		found := false
		for _, v := range matchedIndices {
			if t == v[1] {
				found = true
				break
			}
		}
		if !found {
			unmatchedTrackers = append(unmatchedTrackers, t)
		}
	}

	matches := make([][]int, 0)
	for _, mi := range matchedIndices {
		//filter out matched with low IOU
		iou := 1 - ious[mi[0]][mi[1]]
		if iou < iouThreshold {

			unmatchedDetections = append(unmatchedDetections, mi[0])
			unmatchedTrackers = append(unmatchedTrackers, mi[1])
		} else {
			matches = append(matches, []int{mi[0], mi[1]})
		}
	}

	return matches, unmatchedDetections, unmatchedTrackers
}
