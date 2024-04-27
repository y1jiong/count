package controller

import (
	"count/internal/cache"
	"count/internal/service"
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
	UserId     int64  `json:"user_id"`
	GroupId    int64  `json:"group_id"`
	Message    string `json:"message"`
	Remain     string `json:"remain"`
	CountLimit int    `json:"count_limit"`
	OpenedAt   string `json:"opened_at"`
	ClosedAt   string `json:"closed_at"`
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
	req := &countReq{}
	body, _ := io.ReadAll(r.Body)
	err := json.Unmarshal(body, &req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		g.Log().Notice(ctx, err)
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

	if req.OpenedAt != "" && req.ClosedAt != "" {
		var openedDuration, closedDuration time.Duration
		openedDuration, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.OpenedAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.Log().Notice(ctx, err)
			return
		}
		closedDuration, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.ClosedAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			g.Log().Notice(ctx, err)
			return
		}
		now := gtime.Now()
		nowDuration := now.Sub(now.StartOfDay())
		if nowDuration < openedDuration {
			respContent = "早啊，不过现在好像没到点儿诶"
			return
		}
		if nowDuration > closedDuration {
			respContent = "不早了，现在好像已经过点儿了诶"
			return
		}
	}

	req.Message = strings.TrimSpace(req.Message)
	req.Remain = strings.TrimSpace(req.Remain)

	abbr := req.Message[0 : len(req.Message)-len(req.Remain)]

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
		data = &countCache{
			UserId:  req.UserId,
			GroupId: req.GroupId,
		}
	}

	func() {
		switch {
		case req.Remain == "j":
			if cacheVal == nil {
				respContent = "还没人说过" + service.MapAbbr(ctx, abbr) + "有多少人哦"
				return
			}
			respContent = fmt.Sprintf("%v 在 %v 说%v有 %v 人",
				data.UserId,
				data.Time.Time.Format("2006-01-02 15:04:05"),
				service.MapAbbr(ctx, abbr),
				data.Count,
			)
		case req.Remain == "++":
			data.Count++
			if req.CountLimit > 0 && data.Count > req.CountLimit {
				respContent = "真的有这么多人？！"
				return
			}
			data.Time = gtime.Now()
			err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			respContent = fmt.Sprintf("%v人数加一！现在还有 %v 人",
				service.MapAbbr(ctx, abbr),
				data.Count,
			)
		case req.Remain == "--":
			if data.Count <= 0 {
				respContent = "诶？是不是已经没人排队了"
				return
			}
			data.Count--
			data.Time = gtime.Now()
			err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			respContent = fmt.Sprintf("%v人数减一！现在还有 %v 人",
				service.MapAbbr(ctx, abbr),
				data.Count,
			)
		case strings.HasPrefix(req.Remain, "+"):
			num := gconv.Int(req.Remain[1:])
			if num <= 0 {
				respContent = "啊啊啊，我怎么看不懂你写的是什么"
				return
			}
			data.Count += num
			if req.CountLimit > 0 && data.Count > req.CountLimit {
				respContent = "真的有这么多人？！"
				return
			}
			data.Time = gtime.Now()
			err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			respContent = fmt.Sprintf("%v人数加 %v！现在还有 %v 人",
				service.MapAbbr(ctx, abbr),
				num,
				data.Count,
			)
		case strings.HasPrefix(req.Remain, "-"):
			num := gconv.Int(req.Remain[1:])
			if num <= 0 {
				respContent = "啊啊啊，我怎么看不懂你写的是什么"
				return
			}
			if data.Count < num {
				respContent = "人数不够减哦"
				return
			}
			data.Count -= num
			data.Time = gtime.Now()
			err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			respContent = fmt.Sprintf("%v人数减 %v！现在还有 %v 人",
				service.MapAbbr(ctx, abbr),
				num,
				data.Count,
			)
		default:
			var num int
			num, err = strconv.Atoi(req.Remain)
			if err != nil {
				respContent = "啊啊啊，我怎么看不懂你写的是什么"
				return
			}
			if req.CountLimit > 0 && num > req.CountLimit {
				respContent = "真的有这么多人？！"
				return
			}
			data.Count = num
			data.Time = gtime.Now()
			err = cache.Set(ctx, cacheKey, data, data.Time.EndOfDay().Sub(data.Time))
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			respContent = fmt.Sprintf("我知道%v有 %v 人啦！",
				service.MapAbbr(ctx, abbr),
				num,
			)
		}
	}()
}
