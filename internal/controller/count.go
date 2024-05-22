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
	Message     string `json:"message"`
	Remain      string `json:"remain"`
	OpenAt      string `json:"open_at"`
	CloseAt     string `json:"close_at"`
	ResetAt     string `json:"reset_at"`
	FullName    string `json:"full_name"`
	Note        string `json:"note"`
	Nickname    string `json:"nickname"`
	UserId      int64  `json:"user_id"`
	GroupId     int64  `json:"group_id"`
	CountLimit  int    `json:"count_limit"`
	CostMinutes int    `json:"cost_minutes"`
	CostPer     int    `json:"cost_per"`
}

type countCache struct {
	Nickname string      `json:"nickname"`
	UserId   int64       `json:"user_id"`
	GroupId  int64       `json:"group_id"`
	Count    int         `json:"count"`
	Time     *gtime.Time `json:"time"`
}

var (
	hourAndMinuteRe    = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)
	remainValidationRe = regexp.MustCompile(`^(?:j|几|\+\+|--|\+\d+|-\d+|\d+)$`)
)

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

	req.Remain = strings.TrimSpace(req.Remain)

	if !remainValidationRe.MatchString(req.Remain) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	req.Message = strings.TrimSpace(req.Message)
	abbr := req.Message[0 : len(req.Message)-len(req.Remain)]
	if req.FullName == "" {
		req.FullName = abbr
	}

	if req.OpenAt != "" && req.CloseAt != "" {
		var openDur, closeDur time.Duration
		openDur, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.OpenAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		closeDur, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.CloseAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		now := gtime.Now()
		nowDur := now.Sub(now.StartOfDay())
		if nowDur < openDur {
			respContent = fmt.Sprintf("早啊，不过现在好像没到点儿诶\n%v%v",
				req.FullName,
				req.Note,
			)
			return
		}
		if nowDur > closeDur {
			respContent = fmt.Sprintf("不早了，现在好像已经过点儿了诶\n%v%v",
				req.FullName,
				req.Note,
			)
			return
		}
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
		if req.ResetAt != "" {
			var resetDur time.Duration
			resetDur, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.ResetAt, "${1}h${2}m"))
			if err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			now := gtime.Now()
			resetTime := now.StartOfDay().Add(resetDur)
			if now.After(resetTime) && data.Time.Before(resetTime) {
				data = nil
				cacheVal = nil
				_ = cache.Remove(ctx, cacheKey)
			}
		}
	}
	if data == nil {
		data = &countCache{}
	}

	switch {
	case req.Remain == "j", req.Remain == "几":
		if cacheVal == nil {
			respContent = fmt.Sprintf("还没人说过%v有多少人哦", req.FullName)
			if req.Note != "" {
				respContent += "\n" + req.Note
			}
			return
		}

		respContent = fmt.Sprintf("%v有 %v 人 (%v 分钟前)\n%v(%v) 在 %v 说的",
			req.FullName,
			data.Count,
			gtime.Now().Sub(data.Time).Round(time.Minute).Minutes(),
			data.Nickname,
			data.UserId,
			data.Time.Time.Format("2006-01-02 15:04:05"),
		)

		if req.Note != "" {
			respContent += "\n" + req.Note
		}

		expectWait := data.Count
		if req.CostPer > 0 {
			expectWait /= req.CostPer
		}
		expectWait *= req.CostMinutes

		if expectWait > 0 {
			respContent += fmt.Sprintf("\n预计等待 %v 分钟", expectWait)
		}
	case req.Remain == "++":
		data.Count++
		if req.CountLimit > 0 && data.Count > req.CountLimit {
			respContent = "真的有这么多人？！"
			return
		}

		data.Nickname = req.Nickname
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
		data.Nickname = req.Nickname
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

		data.Nickname = req.Nickname
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
		data.Nickname = req.Nickname
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
		data.Nickname = req.Nickname
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
