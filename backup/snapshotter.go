package backup

import (
	"context"
	"es-backup/config"
	"es-backup/notify"
	"github.com/c2h5oh/datasize"
	es "github.com/elastic/go-elasticsearch/v7"
	"github.com/elastic/go-elasticsearch/v7/esapi"
	"github.com/hako/durafmt"
	"gopkg.in/yaml.v2"
	"log"
	"strconv"
	"strings"
	"time"
)

type Snapshotter struct {
	p *config.Params
	m *notify.Mattermost
	c *es.Client
}

type snapshotStatusResponseBody struct {
	Snapshots []snapshotStatus `yaml:"snapshots"`
}

type snapshotStatus struct {
	Snapshot string `yaml:"snapshot"`
	State    string `yaml:"state"`
	Stats    struct {
		Incremental struct {
			FileCount   int `yaml:"file_count"`
			SizeInBytes int `yaml:"size_in_bytes"`
		} `yaml:"incremental"`
		Total struct {
			FileCount   int `yaml:"file_count"`
			SizeInBytes int `yaml:"size_in_bytes"`
		} `yaml:"total"`
		StartTimeInMillis int `yaml:"start_time_in_millis"`
		TimeInMillis      int `yaml:"time_in_millis"`
	} `yaml:"stats"`
}

type stats struct {
	Incremental struct {
		FileCount   int `yaml:"file_count"`
		SizeInBytes int `yaml:"size_in_bytes"`
	} `yaml:"incremental"`
	Total struct {
		FileCount   int `yaml:"file_count"`
		SizeInBytes int `yaml:"size_in_bytes"`
	} `yaml:"total"`
	StartTimeInMillis int `yaml:"start_time_in_millis"`
	TimeInMillis      int `yaml:"time_in_millis"`
}

type formattedStats struct {
	Incremental struct {
		FileCount int    `yaml:"file_count"`
		Size      string `yaml:"size"`
	} `yaml:"incremental"`
	Total struct {
		FileCount int    `yaml:"file_count"`
		Size      string `yaml:"size"`
	} `yaml:"total"`
	StartTime string `yaml:"start_time"`
	Time      string `yaml:"time"`
}

type snapshotsResponseBody struct {
	Snapshots []snapshot `yaml:"snapshots"`
}

type snapshot struct {
	Snapshot          string `yaml:"snapshot"`
	State             string `yaml:"state"`
	StartTimeInMillis string `yaml:"start_time_in_millis"`
	EndTimeInMillis   string `yaml:"end_time_in_millis"`
}

func NewSnapshotter(params *config.Params) (s *Snapshotter) {
	s = &Snapshotter{
		p: params,
		m: notify.NewMattermost(params),
	}

	c, err := es.NewClient(es.Config{
		Addresses: []string{s.p.Elasticsearch.Url},
		Username:  s.p.Elasticsearch.Username,
		Password:  s.p.Elasticsearch.Password,
		APIKey:    s.p.Elasticsearch.ApiKey,
	})
	if err != nil {
		s.m.Notify(
			"Snapshot alma işleminde hata oluştu.",
			"Elasticsearch client oluşturulamadı:",
			err.Error(),
			true,
		)

		notify.Email(
			s.p,
			"Snapshot alma işleminde hata oluştu",
			"Elasticsearch client oluşturulamadı: "+err.Error(),
			true,
		)

		return
	}
	s.c = c

	return
}

func (s *Snapshotter) Snapshot() {
TASKS:
	for _, task := range s.p.Elasticsearch.Tasks {
		if len(task.Indexes) == 0 {
			s.m.Notify("Elasticsearch için `"+task.Repository+"` repository'sine tüm index'ler için snapshot alma işlemi başladı.", "", "", false)
		} else {
			s.m.Notify("Elasticsearch için `"+task.Repository+"` repository'sine `"+strings.Join(task.Indexes, ",")+"` index'leri için snapshot alma işlemi başladı.", "", "", false)
		}

		vrReq := esapi.SnapshotVerifyRepositoryRequest{
			Repository: task.Repository,
		}
		res, err := vrReq.Do(context.Background(), s.c)
		if err != nil {
			s.m.Notify(
				"Snapshot alma işleminde hata oluştu.",
				"`"+task.Repository+"` repository'si için doğrulama isteği Elasticsearch API'ına gönderilemedi:",
				err.Error(),
				true,
			)
			notify.Email(s.p, "Snapshot alma işleminde hata oluştu.", task.Repository+" repository'si için doğrulama isteği Elasticsearch API'ına gönderilemedi: "+err.Error(), true)
			continue TASKS
		}
		if res.IsError() {
			s.m.Notify(
				"Snapshot alma işleminde hata oluştu.",
				"`"+task.Repository+"` repository'si için doğrulama isteğine Elasticsearch API'ından 2XX harici yanıt geldi:",
				res.String(),
				true,
			)
			notify.Email(s.p, "Snapshot alma işleminde hata oluştu.", task.Repository+" repository'si için doğrulama isteğine Elasticsearch API'ından 2XX harici yanıt geldi: "+res.String(), true)
			continue TASKS
		}

		snapshotName := task.SnapshotName + "--" + time.Now().Format("2006-01-02")

		body := `{"indices": "` + strings.Join(task.Indexes, ",") + `", "metadata":{"taken_by":"` + task.TakenBy + `","taken_because":"` + task.TakenBecause + `"}}`
		if len(task.Indexes) == 0 {
			body = `{"metadata":{"taken_by":"` + task.TakenBy + `","taken_because":"` + task.TakenBecause + `"}}`
		}

		scReq := esapi.SnapshotCreateRequest{
			Body:       strings.NewReader(body),
			Repository: task.Repository,
			Snapshot:   snapshotName,
		}

		res, err = scReq.Do(context.Background(), s.c)
		if err != nil {
			s.m.Notify(
				"Snapshot alma işleminde hata oluştu.",
				"Snapshot oluşturma isteği Elasticsearch API'ına gönderilemedi:",
				err.Error(),
				true,
			)
			notify.Email(s.p, "Snapshot alma işleminde hata oluştu.", "Snapshot oluşturma isteği Elasticsearch API'ına gönderilemedi:"+err.Error(), true)
			continue TASKS
		}
		if res.IsError() {
			s.m.Notify(
				"Snapshot alma işleminde hata oluştu.",
				"Snapshot oluşturma isteğine Elasticsearch API'ından 2XX harici yanıt geldi:",
				res.String(),
				true,
			)
			notify.Email(s.p, "Snapshot alma işleminde hata oluştu.", "Snapshot oluşturma isteğine Elasticsearch API'ından 2XX harici yanıt geldi:" + res.String(), true)
			continue TASKS
		}

		ctx, cancel := context.WithTimeout(context.Background(), time.Duration(task.TimeoutByMinutes)*time.Minute)
		defer cancel()

		for {
			select {
			case <-ctx.Done():
				s.m.Notify(
					"Snapshot alma işleminde hata oluştu.",
					"Snapshot oluşturma isteği sonrası kontrol, zaman aşımına uğradı.",
					"Zaman aşımı süresi: "+strconv.Itoa(task.TimeoutByMinutes)+" dakika",
					true,
				)
				notify.Email(s.p, "Snapshot alma işleminde hata oluştu.", "Snapshot oluşturma isteği sonrası kontrol, zaman aşımına uğradı.", true)
				cancel()
				continue TASKS
			default:
				time.Sleep(5 * time.Second)

				ssReq := esapi.SnapshotStatusRequest{
					Repository: task.Repository,
					Snapshot:   []string{snapshotName},
				}

				res, err = ssReq.Do(ctx, s.c)
				if err != nil {
					log.Printf("Error on checking snapshot status: %q", err.Error())
				}

				ssrb := new(snapshotStatusResponseBody)
				err = yaml.NewDecoder(res.Body).Decode(&ssrb)
				if err != nil {
					log.Printf("Error on decoding snapshot status into struct: %q", err.Error())
				}

				if ssrb.Snapshots[0].Snapshot == snapshotName && ssrb.Snapshots[0].State == "SUCCESS" {
					bs, err := yaml.Marshal(ssrb.Snapshots[0].Stats)
					if err != nil {
						log.Printf("Error on marshalling snapshot status into YAML: %q", err.Error())
					}

					fs, err := formatStats(string(bs))
					if err != nil {
						log.Printf("Error on formatting snapshot stats: %q", err.Error())
					}

					s.m.Notify(
						"Elasticsearch için snapshot alma işlemi sona erdi.",
						"Snapshot ayrıntıları:",
						fs,
						false,
					)
					notify.Email(s.p, "Elasticsearch için snapshot alma işlemi sona erdi.", "Snapshot ayrıntıları: "+fs, false)

					continue TASKS
				}
			}
		}
	}
}

func formatStats(yml string) (string, error) {
	s := stats{}
	err := yaml.Unmarshal([]byte(yml), &s)
	if err != nil {
		return "", err
	}

	fs := formattedStats{}

	fs.Incremental.FileCount = s.Incremental.FileCount
	fs.Incremental.Size = datasize.ByteSize.HR(datasize.ByteSize(s.Incremental.SizeInBytes))
	fs.Total.FileCount = s.Total.FileCount
	fs.Total.Size = datasize.ByteSize.HR(datasize.ByteSize(s.Total.SizeInBytes))

	loc, err := time.LoadLocation("Europe/Istanbul")
	if err != nil {
		return "", err
	}
	st := time.Unix(0, int64(s.StartTimeInMillis)*int64(time.Millisecond)).In(loc)
	fs.StartTime = st.Format("2006-01-02 15:04:05.000")

	fs.Time = durafmt.Parse(time.Duration(s.TimeInMillis) * time.Millisecond).String()

	bytes, err := yaml.Marshal(fs)
	if err != nil {
		return "", err
	}

	return string(bytes), err
}
