package poolsuite

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/effects"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
)

type Track struct {
	SCID   int64  `json:"soundcloud_id"`
	Artist string `json:"artist"`
	Title  string `json:"title"`
}

type Playlist struct {
	Name        string  `json:"name"`
	Slug        string  `json:"slug"`
	Order       int     `json:"order"`
	IsCustom    bool    `json:"isCustom"`
	TotalTracks int     `json:"total_tracks"`
	Tracks      []Track `json:"tracks_in_order"`
}

type TracksByPlaylist struct {
	Playlists []Playlist `json:"payload"`
}

type Poolsuite struct {
	tracksByPlaylist TracksByPlaylist

	streamer beep.StreamSeekCloser
	format   beep.Format
	control  *beep.Ctrl

	volumeLevel int
	volume      *effects.Volume

	refreshTicker *time.Ticker
	progressText  string

	playingTrack *Track
	isPlaying    bool
}

// https://github.com/jypelle/mifasol/blob/808d6637ce592f34b1f57b70f18885fd1f9f9009/internal/cli/ui/playerComponent_amd64.go

func NewPoolsuite() *Poolsuite {
	ps := new(Poolsuite)

	var sampleRate beep.SampleRate = 44100
	speaker.Init(sampleRate, sampleRate.N(time.Second/10))

	ps.SetVolume(100)

	ps.refreshTicker = time.NewTicker(time.Second)

	go func() {
		for {
			select {
			case <-ps.refreshTicker.C:
				ps.refreshProgress()
			}
		}
	}()

	return ps
}

func (ps *Poolsuite) SetVolume(volume int) {
	if volume > 120 {
		ps.volumeLevel = 120
	} else if volume < 0 {
		ps.volumeLevel = 0
	} else {
		ps.volumeLevel = volume
	}

	speaker.Lock()
	if ps.volume != nil {
		ps.volume.Volume = float64(ps.volumeLevel-100) / 16
		ps.volume.Silent = ps.volumeLevel == 0
	}
	speaker.Unlock()
}

func (ps *Poolsuite) PauseResume() {
	if ps.playingTrack != nil {
		speaker.Lock()
		if ps.control != nil {
			ps.control.Paused = !ps.control.Paused
		}
		speaker.Unlock()
		if ps.control.Paused {
			ps.isPlaying = false
		} else {
			ps.isPlaying = true
		}
	}
}

func (ps *Poolsuite) Load() error {
	var err error

	ps.tracksByPlaylist, err = getTracksByPlaylist()
	if err != nil {
		return err
	}

	return nil
}

func (ps *Poolsuite) ListPlaylists() *[]Playlist {
	return &ps.tracksByPlaylist.Playlists
}

func (ps *Poolsuite) GetPlaylistBySlug(slug string) *Playlist {
	for i := 0; i < len(ps.tracksByPlaylist.Playlists); i++ {
		playlist := &ps.tracksByPlaylist.Playlists[i]
		if strings.ToLower(playlist.Slug) == strings.ToLower(slug) {
			return playlist
		}
	}
	return nil
}

func (ps *Poolsuite) GetRandomPlaylist() *Playlist {
	l := len(ps.tracksByPlaylist.Playlists) - 1
	if l <= 0 {
		return nil
	}

	playlistIndex := ps.getRandomBetween(0, l)
	return &ps.tracksByPlaylist.Playlists[playlistIndex]
}

func (ps *Poolsuite) GetRandomTrackFromPlaylist(playlist *Playlist) *Track {
	if playlist == nil {
		return nil
	}

	l := len(playlist.Tracks) - 1
	if l <= 0 {
		return nil
	}

	trackIndex := ps.getRandomBetween(0, l)
	return &playlist.Tracks[trackIndex]
}

func (ps *Poolsuite) getRandomBetween(min, max int) int {
	rand.Seed(time.Now().UnixNano())
	return rand.Intn(max-min+1) + min
}

func (ps *Poolsuite) Play(track *Track, callback func()) error {
	if track == nil {
		return errors.New("No track given")
	}

	ps.playingTrack = track

	speaker.Clear()

	resp, err := http.Get(fmt.Sprintf("https://api.poolsidefm.workers.dev/v2/get_sc_mp3_stream?track_id=%d", track.SCID))
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	// speaker.Lock()
	ps.streamer, ps.format, err = mp3.Decode(resp.Body)
	if err != nil {
		return err
	}

	if ps.format.SampleRate == 44100 {
		ps.control = &beep.Ctrl{Streamer: ps.streamer, Paused: false}
	} else {
		ps.control = &beep.Ctrl{Streamer: beep.Resample(4, ps.format.SampleRate, 44100, ps.streamer), Paused: false}
	}

	ps.volume = &effects.Volume{
		Streamer: ps.control,
		Base:     2,
		Volume:   float64(ps.volumeLevel-100) / 16,
		Silent:   ps.volumeLevel == 0,
	}

	speaker.Play(
		beep.Seq(
			ps.volume,
			beep.Callback(
				func() {
					ps.refreshProgress()
					ps.control = nil
					ps.volume = nil
					ps.streamer = nil
					if callback != nil {
						callback()
					}
				},
			),
		),
	)

	ps.isPlaying = true

	return nil
}

func getTracksByPlaylist() (TracksByPlaylist, error) {
	var client = &http.Client{Timeout: 10 * time.Second}
	r, err := client.Get(
		"https://api.poolsidefm.workers.dev/v1/get_tracks_by_playlist",
	)
	if err != nil {
		return TracksByPlaylist{}, err
	}
	defer r.Body.Close()
	var result TracksByPlaylist
	json.NewDecoder(r.Body).Decode(&result)

	return result, nil
}

func (p *TracksByPlaylist) getTrackFromPlaylist(playlistIndex int, trackIndex int) *Track {
	return &p.Playlists[playlistIndex].Tracks[trackIndex]
}

func (ps *Poolsuite) refreshProgress() {
	speaker.Lock()
	if ps.control != nil {
		duration := ps.format.SampleRate.D(ps.streamer.Position()).Round(time.Second)
		min := duration / time.Minute
		duration -= min * time.Minute
		sec := duration / time.Second

		duration = ps.format.SampleRate.D(ps.streamer.Len()).Round(time.Second)
		totalMin := duration / time.Minute
		duration -= totalMin * time.Minute
		totalSec := duration / time.Second

		ps.progressText = fmt.Sprintf("%02d:%02d / %02d:%02d", min, sec, totalMin, totalSec)
	}
	speaker.Unlock()
}

