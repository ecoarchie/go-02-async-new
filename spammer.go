package main

import (
	"fmt"
	"sort"
	"strconv"
	"sync"
)

func RunPipeline(cmds ...cmd) {
	wg := &sync.WaitGroup{}
	in := make(chan interface{})
	for _, c := range cmds {
		wg.Add(1)
		out := make(chan interface{})
		go func(command cmd, wait *sync.WaitGroup, in, out chan interface{}) {
			defer wait.Done()

			command(in, out)
			close(out)
		}(c, wg, in, out)
		in = out
	}
	wg.Wait()
}

func SelectUsers(in, out chan interface{}) {
	wg := sync.WaitGroup{}
	mu := sync.Mutex{}
	set := make(map[string]bool)

	for email := range in {
		wg.Add(1)
		go func(email string) {
			e := GetUser(email)
			mu.Lock()
			isPresent := set[e.Email]
			if !isPresent {
				out <- e
				set[e.Email] = true
			}
			mu.Unlock()
			wg.Done()
		}(email.(string))
	}
	wg.Wait()
	clear(set)
}

func GetMessagesForUsers(users []User, out chan interface{}, wg *sync.WaitGroup) {
	msgs, err := GetMessages(users...)
	if err != nil {
		fmt.Printf("Error getting messages: %v\n", err)
		return
	}
	for _, msg := range msgs {
		out <- msg
	}
	wg.Done()
}

func SelectMessages(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	users := []User{}
	for u := range in {
		user := u.(User)
		users = append(users, user)
		if len(users) == GetMessagesMaxUsersBatch {
			wg.Add(1)
			go GetMessagesForUsers(users, out, wg)
			users = nil
		}
	}
	if len(users) != 0 {
		wg.Add(1)
		go GetMessagesForUsers(users, out, wg)
	}
	wg.Wait()
}

func CheckSpam(in, out chan interface{}) {
	wg := &sync.WaitGroup{}
	quotaChan := make(chan struct{}, HasSpamMaxAsyncRequests)
	for msg := range in {
		wg.Add(1)
		msgID := msg.(MsgID)

		go func(id MsgID, quota chan struct{}, wg *sync.WaitGroup) {
			defer wg.Done()

			quota <- struct{}{}
			defer func() { <-quota }()
			hasSpam, err := HasSpam(id)
			if err != nil {
				fmt.Printf("error checking for spam: %v\n", err)
				return
			}

			msgData := MsgData{ID: id, HasSpam: hasSpam}
			out <- msgData
		}(msgID, quotaChan, wg)
	}
	wg.Wait()
}

func CombineResults(in, out chan interface{}) {
	trues := []MsgData{}
	falses := []MsgData{}
	results := []string{}
	for s := range in {
		v := s.(MsgData)
		if v.HasSpam {
			trues = append(trues, v)
		} else {
			falses = append(falses, v)
		}
	}
	sort.Slice(trues, func(i, j int) bool {
		return trues[i].ID < trues[j].ID
	})
	sort.Slice(falses, func(i, j int) bool {
		return falses[i].ID < falses[j].ID
	})

	for _, v := range trues {
		id := strconv.FormatUint(uint64(v.ID), 10)
		b := strconv.FormatBool(v.HasSpam)
		results = append(results, b + " " + id)

	}
	for _, v := range falses {
		id := strconv.FormatUint(uint64(v.ID), 10)
		b := strconv.FormatBool(v.HasSpam)
		results = append(results, b + " " + id)

	}
	for _, b := range results {
		out <- b
	}
}
