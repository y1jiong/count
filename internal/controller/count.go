package controller

import (
	"count/internal/cache"
	"encoding/json"
	"fmt"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type countReq struct {
	Message    string `json:"message"`
	Remain     string `json:"remain"`
	OpenAt     string `json:"open_at"`
	CloseAt    string `json:"close_at"`
	FullName   string `json:"full_name"`
	UserId     int64  `json:"user_id"`
	GroupId    int64  `json:"group_id"`
	CountLimit int    `json:"count_limit"`
}

type countCache struct {
	UserId  int64       `json:"user_id"`
	GroupId int64       `json:"group_id"`
	Count   int         `json:"count"`
	Time    *gtime.Time `json:"time"`
}

var hourAndMinuteRe = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)

func Count(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, _ := io.ReadAll(r.Body)
	var req *countReq
	err := json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var respContent string
	defer func() {
		if respContent == "" {
			return
		}
		_, err = w.Write([]byte(respContent))
		if err != nil {
			g.Log().Error(ctx, err)
			return
		}
	}()

	if req.OpenAt != "" && req.CloseAt != "" {
		var openDuration, closeDuration time.Duration
		openDuration, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.OpenAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		closeDuration, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.CloseAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		now := gtime.Now()
		nowDuration := now.Sub(now.StartOfDay())
		if nowDuration < openDuration {
			respContent = "早啊，不过现在好像没到点儿诶"
			return
		}
		if nowDuration > closeDuration {
			respContent = "不早了，现在好像已经过点儿了诶"
			return
		}
	}

	req.Message = strings.TrimSpace(req.Message)
	req.Remain = strings.TrimSpace(req.Remain)

	abbr := req.Message[0 : len(req.Message)-len(req.Remain)]
	if req.FullName == "" {
		req.FullName = abbr
	}

	cacheKey := "count_" + abbr
	cacheVal, err := cache.Get(ctx, cacheKey)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		g.Log().Error(ctx, err)
		return
	}

	var data *countCache
	if cacheVal != nil {
		err = cacheVal.Scan(&data)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}
		if data.Time.Time.Format("2006-01-02") != gtime.Now().Time.Format("2006-01-02") {
			data = nil
			cacheVal = nil
			_ = cache.Remove(ctx, cacheKey)
		}
	}
	if data == nil {
		data = &countCache{}
	}

	switch {
	case req.Remain == "j", req.Remain == "几":
		if cacheVal == nil {
			respContent = "还没人说过" + req.FullName + "有多少人哦"
			return
		}

		respContent = fmt.Sprintf("%v 在 %v 说%v有 %v 人",
			data.UserId,
			data.Time.Time.Format("2006-01-02 15:04:05"),
			req.FullName,
			data.Count,
		)
	case req.Remain == "++":
		data.Count++
		if req.CountLimit > 0 && data.Count > req.CountLimit {
			respContent = "真的有这么多人？！"
			return
		}

		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数加一！现在有 %v 人",
			req.FullName,
			data.Count,
		)
	case req.Remain == "--":
		if data.Count <= 0 {
			respContent = "诶？是不是已经没人排队了"
			return
		}

		data.Count--
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数减一！现在有 %v 人",
			req.FullName,
			data.Count,
		)
	case strings.HasPrefix(req.Remain, "+"):
		num := gconv.Int(req.Remain[1:])
		if num <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		data.Count += num
		if req.CountLimit > 0 && data.Count > req.CountLimit {
			respContent = "真的有这么多人？！"
			return
		}

		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数加 %v！现在有 %v 人",
			req.FullName,
			num,
			data.Count,
		)
	case strings.HasPrefix(req.Remain, "-"):
		num := gconv.Int(req.Remain[1:])
		if num <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if data.Count < num {
			respContent = "人数不够减哦"
			return
		}

		data.Count -= num
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数减 %v！现在有 %v 人",
			req.FullName,
			num,
			data.Count,
		)
	default:
		var num int
		num, err = strconv.Atoi(req.Remain)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.CountLimit > 0 && num > req.CountLimit {
			respContent = "真的有这么多人？！"
			return
		}

		data.Count = num
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("我知道%v有 %v 人啦！",
			req.FullName,
			num,
		)
	}
}
