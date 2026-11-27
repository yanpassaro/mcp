package sandbox

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

const (
	JOB_KEEP      = 50
	JOB_RUNNING   = "running"
	JOB_DONE      = "done"
	JOB_FAILED    = "failed"
	JOB_CANCELLED = "cancelled"
)

type JobInfo struct {
	ID          string
	Name        string
	Description string
	Status      string
	Started     time.Time
	Ended       time.Time
	Output      string
	Truncated   bool
	Result      RunResult
	Err         error
}

type Job struct {
	id          string
	name        string
	description string
	status      string
	started     time.Time
	ended       time.Time
	mu          sync.Mutex
	output      strings.Builder
	truncated   bool
	result      RunResult
	err         error
}

type Jobs struct {
	mu    sync.Mutex
	next  int
	all   map[string]*Job
	order []string
}

func NewJobs() *Jobs {
	return &Jobs{all: map[string]*Job{}}
}

func (j *Jobs) Start(store, tmp *Store, r RunRequest) JobInfo {
	j.mu.Lock()
	j.next++
	id := fmt.Sprintf("job-%d", j.next)
	job := newJob(id, r.Code)
	j.all[id] = job
	j.order = append(j.order, id)
	j.prune()
	j.mu.Unlock()

	go j.run(store, tmp, r, job)
	return job.info()
}

func (j *Jobs) run(store, tmp *Store, r RunRequest, job *Job) {
	r.Sink = job.write
	res, err := Run(store, tmp, r)
	job.finish(res, err)
}

func (j *Jobs) Get(id string) (JobInfo, bool) {
	job := j.find(id)
	if job == nil {
		return JobInfo{}, false
	}
	return job.info(), true
}

func (j *Jobs) List() []JobInfo {
	j.mu.Lock()
	ids := make([]string, len(j.order))
	copy(ids, j.order)
	j.mu.Unlock()

	infos := make([]JobInfo, 0, len(ids))
	for i := len(ids) - 1; i >= 0; i-- {
		job := j.find(ids[i])
		if job == nil {
			continue
		}
		infos = append(infos, job.info())
	}
	return infos
}

func (j *Jobs) Kill(id string) (JobInfo, bool) {
	job := j.find(id)
	if job == nil {
		return JobInfo{}, false
	}
	job.cancel()
	return job.info(), true
}

func (j *Jobs) find(id string) *Job {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.all[id]
}

func (j *Jobs) prune() {
	for len(j.order) > JOB_KEEP {
		id := j.order[0]
		job := j.all[id]
		if job.running() {
			return
		}
		delete(j.all, id)
		j.order = j.order[1:]
	}
}

func newJob(id, code string) *Job {
	name, desc := ParseMeta(code)
	return &Job{
		id:          id,
		name:        name,
		description: desc,
		status:      JOB_RUNNING,
		started:     time.Now(),
	}
}

func (job *Job) write(s string) {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.output.Len() >= MAX_OUTPUT_BYTES {
		job.truncated = true
		return
	}
	remaining := MAX_OUTPUT_BYTES - job.output.Len()
	if len(s) > remaining {
		s = s[:remaining]
		job.truncated = true
	}
	job.output.WriteString(s)
}

func (job *Job) finish(res RunResult, err error) {
	job.mu.Lock()
	defer job.mu.Unlock()
	job.result = res
	job.err = err
	job.ended = time.Now()
	if job.status == JOB_CANCELLED {
		return
	}
	job.status = JOB_DONE
	if err != nil {
		job.status = JOB_FAILED
		return
	}
	if !res.Ok {
		job.status = JOB_FAILED
	}
}

func (job *Job) cancel() {
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.status != JOB_RUNNING {
		return
	}
	job.status = JOB_CANCELLED
	job.ended = time.Now()
}

func (job *Job) running() bool {
	job.mu.Lock()
	defer job.mu.Unlock()
	return job.status == JOB_RUNNING
}

func (job *Job) info() JobInfo {
	job.mu.Lock()
	defer job.mu.Unlock()
	return JobInfo{
		ID:          job.id,
		Name:        job.name,
		Description: job.description,
		Status:      job.status,
		Started:     job.started,
		Ended:       job.ended,
		Output:      job.output.String(),
		Truncated:   job.truncated,
		Result:      job.result,
		Err:         job.err,
	}
}
