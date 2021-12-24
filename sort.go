package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/sirupsen/logrus"
	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
	// "github.com/cpmech/gosl"
)

//SORT Detection tracking
type SORT struct {
	maxPredictsWithoutUpdate int
	minUpdatesUsePrediction  int
	iouThreshold             float64
	Trackers                 []*KalmanBoxTracker
	FrameCount               int
}

//NewSORT initializes a new SORT tracking session
func NewSORT(maxPredictsWithoutUpdate int, minUpdatesUsePrediction int, iouThreshold float64) SORT {
	return SORT{
		maxPredictsWithoutUpdate: maxPredictsWithoutUpdate,
		minUpdatesUsePrediction:  minUpdatesUsePrediction,
		iouThreshold:             iouThreshold,
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

func (s *SORT) Updatez(items []gjson.Result, width float64, height float64, fo *os.File) ([]string, error) {

	dets := [][]float64{}
	for _, item := range items {
		bbox := []float64{}
		item.Get("bbox").ForEach(func(key, value gjson.Result) bool {
			bbox = append(bbox, value.Num)
			return true
		})
		bbox = []float64{bbox[0] * width, bbox[1] * height, bbox[0]*width + bbox[2]*width, bbox[1]*height + bbox[3]*height}
		bbox_prob := append(bbox, item.Get("prob").Num)
		dets = append(dets, bbox_prob)
		// logrus.Debugf("SORT Update dets=%v iouThreshold=%f", bbox_prob, s.iouThreshold)
		fo.WriteString(fmt.Sprintf("SORT Update dets=%v iouThreshold=%f \n", bbox_prob, s.iouThreshold))
	}
	fo.WriteString("check after get detections \n")

	items_string := []string{}
	for _, item := range items {
		items_string = append(items_string, item.String())
	}
	fo.WriteString("check after get items \n")

	// func (s *SORT) Updatez(dets [][]float64) error {
	// 	logrus.Debugf("SORT Update dets=%v iouThreshold=%f", dets, s.iouThreshold)
	s.FrameCount = s.FrameCount + 1

	//NOT SURE HOW KALMAN ALGO WILL SHOW ERRORS. SEE LATER AND REMOVE INVALID PREDICTORS
	// trks := make([]KalmanBoxTracker, 0)
	// for _, v := range s.Trackers {
	// 	trks = append(trks, v)
	// }
	// get predicted locations from existing trackers.
	//     trks = np.zeros((len(self.trackers),5))
	//     to_del = []
	//     ret = []
	//     for t,trk in enumerate(trks):
	//       pos = self.trackers[t].predict()[0]
	//       trk[:] = [pos[0], pos[1], pos[2], pos[3], 0]
	//       if(np.any(np.isnan(pos))):
	//         to_del.append(t)
	//     trks = np.ma.compress_rows(np.ma.masked_invalid(trks))
	//     for t in reversed(to_del):
	//       self.trackers.pop(t)

	matched, unmatchedDets, unmatchedTrks := associateDetectionsToTrackers(dets, s.Trackers, s.iouThreshold, s.minUpdatesUsePrediction, fo)
	// fo.WriteString("check4")

	fo.WriteString(fmt.Sprintf("Detection X Trackers. matched=%v unmatchedDets=%v unmatchedTrks=%v \n", matched, unmatchedDets, unmatchedTrks))
	fo.WriteString("check after get items \n")
	// logrus.Debugf("Detection X Trackers. matched=%v unmatchedDets=%v unmatchedTrks=%v", matched, unmatchedDets, unmatchedTrks)

	// update matched trackers with assigned detections
	for t := 0; t < len(s.Trackers); t++ {
		tracker := s.Trackers[t]
		//is this tracker still matched?
		if !contains(unmatchedTrks, t) {
			for _, det := range matched {
				if det[1] == t {
					bbox := dets[det[0]]
					// _, err :=
					tracker.Update(bbox)
					items_string[det[0]], _ = sjson.Set(items_string[det[0]], "id", strconv.FormatInt(int64(tracker.ID), 10))
					// if err != nil {
					// 	return err
					// }
					logrus.Debugf("Tracker updated. id=%d bbox=%v updates=%d \n", tracker.ID, bbox, tracker.Updates)
					// fo.WriteString(fmt.Sprintf("Tracker updated. id=%d bbox=%v updates=%d\n", tracker.ID, bbox, tracker.Updates))
					break
				}
			}
			// d = matched[np.where(matched[:,1]==t)[0],0]
			// trk.update(dets[d,:][0])
		}
	}
	fo.WriteString("check ater update detections \n")

	// create and initialise new trackers for unmatched detections
	for _, udet := range unmatchedDets {

		// aread := Area(dets[udet])
		// if aread < 1 {
		// 	logrus.Debugf("Ignoring too small detection. bbox=%f area=%f", dets[udet], aread)
		// 	continue
		// }

		trk, _ := NewKalmanBoxTracker(dets[udet])

		s.Trackers = append(s.Trackers, &trk)
		items_string[udet], _ = sjson.Set(items_string[udet], "id", strconv.FormatInt(int64(s.Trackers[len(s.Trackers)-1].ID), 10))
		// logrus.Debugf("New tracker added. id=%d bbox=%v\n", trk.ID, trk.LastBBox)
		fo.WriteString(fmt.Sprintf("New tracker added. id=%d bbox=%v \n", trk.ID, trk.LastBBox))
	}
	fo.WriteString("check after unmatched tracks and add new tracks \n")

	delete_ind := []int{}
	for _, t := range unmatchedTrks {
		trk := s.Trackers[t]
		//         if((trk.time_since_update < 1) and (trk.hit_streak >= self.min_hits or self.frame_count <= self.min_hits)):
		//           ret.append(np.concatenate((d,[trk.id+1])).reshape(1,-1)) # +1 as MOT benchmark requires positive
		if trk.PredictsSinceUpdate > s.maxPredictsWithoutUpdate || trk.SkipPredicts > s.minUpdatesUsePrediction+1 {
			// s.Trackers = append(s.Trackers[:t], s.Trackers[t+1:]...)
			delete_ind = append(delete_ind, t)
			// logrus.Debugf("Tracker removed. id=%d, bbox=%v updates=%d\n", trk.ID, trk.LastBBox, trk.Updates)
			fo.WriteString(fmt.Sprintf("Tracker removed. id=%d, bbox=%v updates=%d \n", trk.ID, trk.LastBBox, trk.Updates))
		}
	}
	fo.WriteString("check after add delete indexes")

	//remove dead trackers
	// ti := len(s.Trackers)
	// for t := ti - 1; t >= 0; t-- {
	// 	trk := s.Trackers[t]
	// 	//         if((trk.time_since_update < 1) and (trk.hit_streak >= self.min_hits or self.frame_count <= self.min_hits)):
	// 	//           ret.append(np.concatenate((d,[trk.id+1])).reshape(1,-1)) # +1 as MOT benchmark requires positive
	// 	if trk.PredictsSinceUpdate > s.maxPredictsWithoutUpdate || trk.SkipPredicts > s.minUpdatesUsePrediction+1 {
	// 		s.Trackers = append(s.Trackers[:t], s.Trackers[t+1:]...)
	// 		logrus.Debugf("Tracker removed. id=%d, bbox=%v updates=%d\n", trk.ID, trk.LastBBox, trk.Updates)
	// 	}
	// }
	if len(delete_ind) > 0 {
		s.RemoveTrackByIndexs(delete_ind)
	}
	ct := ""
	for _, v := range s.Trackers {
		ct = ct + fmt.Sprintf("[id=%d bbox=%v updates=%d] ", v.ID, v.LastBBox, v.Updates)
	}
	// fo.WriteString("check8")

	// logrus.Debugf("Current trackers=%s", ct)
	fo.WriteString(fmt.Sprintf("Current trackers=%s \n", ct))
	return items_string, nil
}

// func (s *SORT) Update(dets [][]float64) error {
// 	logrus.Debugf("SORT Update dets=%v iouThreshold=%f", dets, s.iouThreshold)
// 	s.FrameCount = s.FrameCount + 1

// 	//NOT SURE HOW KALMAN ALGO WILL SHOW ERRORS. SEE LATER AND REMOVE INVALID PREDICTORS
// 	// trks := make([]KalmanBoxTracker, 0)
// 	// for _, v := range s.Trackers {
// 	// 	trks = append(trks, v)
// 	// }
// 	// get predicted locations from existing trackers.
// 	//     trks = np.zeros((len(self.trackers),5))
// 	//     to_del = []
// 	//     ret = []
// 	//     for t,trk in enumerate(trks):
// 	//       pos = self.trackers[t].predict()[0]
// 	//       trk[:] = [pos[0], pos[1], pos[2], pos[3], 0]
// 	//       if(np.any(np.isnan(pos))):
// 	//         to_del.append(t)
// 	//     trks = np.ma.compress_rows(np.ma.masked_invalid(trks))
// 	//     for t in reversed(to_del):
// 	//       self.trackers.pop(t)

// 	matched, unmatchedDets, unmatchedTrks := associateDetectionsToTrackers(dets, s.Trackers, s.iouThreshold, s.minUpdatesUsePrediction)

// 	logrus.Debugf("Detection X Trackers. matched=%v unmatchedDets=%v unmatchedTrks=%v", matched, unmatchedDets, unmatchedTrks)

// 	// update matched trackers with assigned detections
// 	for t := 0; t < len(s.Trackers); t++ {
// 		tracker := s.Trackers[t]
// 		//is this tracker still matched?
// 		if !contains(unmatchedTrks, t) {
// 			for _, det := range matched {
// 				if det[1] == t {
// 					bbox := dets[det[0]]
// 					_, err := tracker.Update(bbox)
// 					if err != nil {
// 						return nil
// 					}
// 					logrus.Debugf("Tracker updated. id=%d bbox=%v updates=%d\n", tracker.ID, bbox, tracker.Updates)
// 					break
// 				}
// 			}
// 			// d = matched[np.where(matched[:,1]==t)[0],0]
// 			// trk.update(dets[d,:][0])
// 		}
// 	}

// 	// create and initialise new trackers for unmatched detections
// 	for _, udet := range unmatchedDets {

// 		// aread := Area(dets[udet])
// 		// if aread < 1 {
// 		// 	logrus.Debugf("Ignoring too small detection. bbox=%f area=%f", dets[udet], aread)
// 		// 	continue
// 		// }

// 		trk, _ := NewKalmanBoxTracker(dets[udet])

// 		s.Trackers = append(s.Trackers, &trk)
// 		logrus.Debugf("New tracker added. id=%d bbox=%v\n", trk.ID, trk.LastBBox)
// 	}

// 	//remove dead trackers
// 	ti := len(s.Trackers)
// 	for t := ti - 1; t >= 0; t-- {
// 		trk := s.Trackers[t]
// 		//         if((trk.time_since_update < 1) and (trk.hit_streak >= self.min_hits or self.frame_count <= self.min_hits)):
// 		//           ret.append(np.concatenate((d,[trk.id+1])).reshape(1,-1)) # +1 as MOT benchmark requires positive
// 		if trk.PredictsSinceUpdate > s.maxPredictsWithoutUpdate || trk.SkipPredicts > s.minUpdatesUsePrediction+1 {
// 			s.Trackers = append(s.Trackers[:t], s.Trackers[t+1:]...)
// 			logrus.Debugf("Tracker removed. id=%d, bbox=%v updates=%d\n", trk.ID, trk.LastBBox, trk.Updates)
// 		}
// 	}

// 	ct := ""
// 	for _, v := range s.Trackers {
// 		ct = ct + fmt.Sprintf("[id=%d bbox=%v updates=%d] ", v.ID, v.LastBBox, v.Updates)
// 	}
// 	logrus.Debugf("Current trackers=%s", ct)

// 	return nil
// }

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
func associateDetectionsToTrackers(detections [][]float64, trackers []*KalmanBoxTracker, iouThreshold float64, minUpdatesUsePrediction int, fo *os.File) ([][]int, []int, []int) {
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

	// iouMatrix := make([][]float64, ld)
	// fo.WriteString("check3.1")
	mk := Munkres{}
	mk.Init(int(ld), int(lt))

	// mm := munkres.NewMatrix(ld, lt)
	//initialize IOUS cost matrix
	ious := make([][]float64, ld)
	for i := 0; i < len(ious); i++ {
		ious[i] = make([]float64, lt)
	}
	// fo.WriteString("check3.2")

	predicted := make([]bool, lt)
	for d := 0; d < ld; d++ {
		// iouMatrix[d] = make([]float64, lt)
		for t := 0; t < lt; t++ {
			trk := trackers[t]

			//use simple last bbox if not enough updates in this tracker
			tbbox := trk.LastBBox

			//use prediction
			if trk.Updates >= minUpdatesUsePrediction {
				//in this frame request, predict just once
				if !predicted[t] {
					tbbox = trk.PredictNext()
					predicted[t] = true
				} else {
					tbbox = trk.CurrentPrediction()
				}
			} else {
				trk.SkipPredicts = trk.SkipPredicts + 1
			}

			// tbbox1 := trk.LastBBox
			// tbbox = ResizeFromCenter(trk.LastBBox, 4.0)
			// fmt.Printf("ioubbox - %v %v", tbbox, tbbox1)
			v := IOU(detections[d], tbbox) //+ AreaMatch(detections[d], tbbox1) + RatioMatch(detections[d], tbbox1)
			trk.LastBBoxIOU = tbbox
			// if v > 0 {
			// logrus.Debugf("IOU=%v detbbox=%v trackerrefbbox=%v trackerid=%d lastbbox=%v", v, detections[d], tbbox, trackers[t].ID, trackers[t].LastBBox)
			fo.WriteString(fmt.Sprintf("IOU=%v detbbox=%v trackerrefbbox=%v trackerid=%d lastbbox=%v", v, detections[d], tbbox, trackers[t].ID, trackers[t].LastBBox))
			// }
			//invert cost matrix (we want max cost here)
			ious[d][t] = 1 - v
		}
	}
	fo.WriteString("check after predictions")

	//calculate best DETECTION vs TRACKER matches according to COST matrix
	mk.SetCostMatrix(ious)
	mk.Run()
	matchedIndices := [][]int{}
	for i, j := range mk.Links {
		if j != -1 {
			matchedIndices = append(matchedIndices, []int{i, j})
		}
	}
	// fo.WriteString("check3.4")

	// logrus.Debugf("Detection x Tracker match=%v", matchedIndices)
	fo.WriteString(fmt.Sprintf("Detection x Tracker match=%v", matchedIndices))

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
			// logrus.Debugf("Unmatched detection found. bbox=%v", detections[d])
			fo.WriteString(fmt.Sprintf("Unmatched detection found. bbox=%v", detections[d]))
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
			// logrus.Debugf("Skipping detection/tracker because it has low IOU deti=%d trki=%d iou=%f", mi[0], mi[1], iou)
			fo.WriteString(fmt.Sprintf("Skipping detection/tracker because it has low IOU deti=%d trki=%d iou=%f", mi[0], mi[1], iou))
			unmatchedDetections = append(unmatchedDetections, mi[0])
			unmatchedTrackers = append(unmatchedTrackers, mi[1])
		} else {
			matches = append(matches, []int{mi[0], mi[1]})
		}
	}
	fo.WriteString("check after fill matched unmatcheddet and unmatchedtrackers")

	return matches, unmatchedDetections, unmatchedTrackers
}
