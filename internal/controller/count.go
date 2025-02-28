package controller

import (
	"count/internal/service/cache"
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
	ResetAt    string `json:"reset_at"`
	FullName   string `json:"full_name"`
	Note       string `json:"note"`
	Nickname   string `json:"nickname"`
	UserId     int64  `json:"user_id"`
	GroupId    int64  `json:"group_id"`
	CountLimit int    `json:"count_limit"`
	CostMinute int    `json:"cost_minute"`
	CostPer    int    `json:"cost_per"`
}

type countCache struct {
	Nickname string      `json:"nickname"`
	Message  string      `json:"message,omitempty"`
	UserId   int64       `json:"user_id"`
	GroupId  int64       `json:"group_id,omitempty"`
	Count    int         `json:"count"`
	Time     *gtime.Time `json:"time"`
}

var (
	hourAndMinuteRe = regexp.MustCompile(`^(\d{1,2}):(\d{1,2})$`)
	remainExtractRe = regexp.MustCompile(`^(j|几个?|\+\+|--|[+-]?\d+)(?:[，,]([\s\S]+))?$`)
)

func Count(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	body, _ := io.ReadAll(r.Body)
	var req *countReq
	if err := json.Unmarshal(body, &req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var respContent string
	defer func() {
		if respContent == "" {
			return
		}
		_, err := w.Write([]byte(respContent))
		if err != nil {
			g.Log().Error(ctx, err)
			return
		}
	}()

	abbr := req.Message[0 : len(req.Message)-len(req.Remain)]
	if req.FullName == "" {
		req.FullName = abbr
	}

	req.Remain = strings.TrimSpace(req.Remain)
	if !remainExtractRe.MatchString(req.Remain) {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	var cmd, msg string
	{
		matches := remainExtractRe.FindStringSubmatch(req.Remain)
		cmd = matches[1]
		if len(matches) > 2 {
			msg = matches[2]
		}
	}

	if req.OpenAt != "" && req.CloseAt != "" {
		var openDur, closeDur time.Duration
		openDur, err := time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.OpenAt, "${1}h${2}m"))
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

	var resetDur time.Duration
	if req.ResetAt != "" {
		var err error
		resetDur, err = time.ParseDuration(hourAndMinuteRe.ReplaceAllString(req.ResetAt, "${1}h${2}m"))
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	cacheKey := "count_" + abbr
	var data *countCache
	{
		cacheVal, err := cache.Get(ctx, cacheKey)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}
		if cacheVal != nil {
			if err = cacheVal.Scan(&data); err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				g.Log().Error(ctx, err)
				return
			}
			if req.ResetAt != "" {
				now := gtime.Now()
				resetTime := now.StartOfDay().Add(resetDur)
				if now.After(resetTime) && data.Time.Before(resetTime) {
					data = nil
					cacheVal = nil
					_ = cache.Remove(ctx, cacheKey)
				}
			}
		}
	}
	if data == nil {
		data = &countCache{}
	}

	const (
		remindMessageText             = "\n可以在命令后面加逗号(，)写上你想加的备注哦~"
		leaveMessageSuccessTextPrefix = "\n嗷呜！我会转达"
		leaveMessageSuccessTextSuffix = "的！"
	)

	switch {
	case cmd == "j", cmd == "几", cmd == "几个":
		if data.UserId == 0 {
			respContent = fmt.Sprintf("还没人说过%v有多少人哦", req.FullName)
			if req.Note != "" {
				respContent += "\n" + req.Note
			}
			return
		}

		respContent = fmt.Sprintf("%v有 %v 人 (%v 分钟前)",
			req.FullName,
			data.Count,
			gtime.Now().Sub(data.Time).Round(time.Minute).Minutes(),
		)
		if data.Message != "" {
			respContent += "\n" + data.Message
		}
		respContent += fmt.Sprintf("\n%v(%v) 在 %v 说的",
			data.Nickname,
			data.UserId,
			data.Time.Time.Format("2006-01-02 15:04:05"),
		)
		if req.Note != "" {
			respContent += "\n" + req.Note
		}

		expectWait := data.Count
		if req.CostPer > 1 {
			expectWait /= req.CostPer
		}
		expectWait *= req.CostMinute

		if expectWait > 0 {
			respContent += fmt.Sprintf("\n预计等待 %v 分钟", expectWait)
		}
	case cmd == "++":
		data.Count++
		if req.CountLimit > 0 && data.Count > req.CountLimit {
			respContent = "真的有这么多人？！"
			return
		}

		data.Nickname = req.Nickname
		data.Message = msg
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err := cache.Set(ctx, cacheKey, data,
			data.Time.AddDate(0, 0, 1).StartOfDay().Add(resetDur).Sub(data.Time),
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数加一！现在有 %v 人",
			req.FullName,
			data.Count,
		)

		if msg == "" {
			respContent += remindMessageText
		} else {
			respContent += leaveMessageSuccessTextPrefix + msg + leaveMessageSuccessTextSuffix
		}
	case cmd == "--":
		if data.Count <= 0 {
			respContent = "诶？是不是已经没人排队了"
			return
		}

		data.Count--
		data.Nickname = req.Nickname
		data.Message = msg
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err := cache.Set(ctx, cacheKey, data,
			data.Time.AddDate(0, 0, 1).StartOfDay().Add(resetDur).Sub(data.Time),
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("%v人数减一！现在有 %v 人",
			req.FullName,
			data.Count,
		)

		if msg == "" {
			respContent += remindMessageText
		} else {
			respContent += leaveMessageSuccessTextPrefix + msg + leaveMessageSuccessTextSuffix
		}
	case strings.HasPrefix(cmd, "+"):
		num := gconv.Int(cmd[1:])
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
		data.Message = msg
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err := cache.Set(ctx, cacheKey, data,
			data.Time.AddDate(0, 0, 1).StartOfDay().Add(resetDur).Sub(data.Time),
		)
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

		if msg == "" {
			respContent += remindMessageText
		} else {
			respContent += leaveMessageSuccessTextPrefix + msg + leaveMessageSuccessTextSuffix
		}
	case strings.HasPrefix(cmd, "-"):
		num := gconv.Int(cmd[1:])
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
		data.Message = msg
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err := cache.Set(ctx, cacheKey, data,
			data.Time.AddDate(0, 0, 1).StartOfDay().Add(resetDur).Sub(data.Time),
		)
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

		if msg == "" {
			respContent += remindMessageText
		} else {
			respContent += leaveMessageSuccessTextPrefix + msg + leaveMessageSuccessTextSuffix
		}
	default:
		num, err := strconv.Atoi(cmd)
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
		data.Message = msg
		data.UserId = req.UserId
		data.GroupId = req.GroupId
		data.Time = gtime.Now()

		err = cache.Set(ctx, cacheKey, data,
			data.Time.AddDate(0, 0, 1).StartOfDay().Add(resetDur).Sub(data.Time),
		)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			g.Log().Error(ctx, err)
			return
		}

		respContent = fmt.Sprintf("我知道%v有 %v 人啦！",
			req.FullName,
			num,
		)

		if msg == "" {
			respContent += remindMessageText
		} else {
			respContent += leaveMessageSuccessTextPrefix + msg + leaveMessageSuccessTextSuffix
		}
	}
}
