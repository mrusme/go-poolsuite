go-poolsuite
------------

[Poolsuite FM](https://poolsuite.net) (formerly 
[Poolside FM](https://github.com/Poolside-FM)) player as a Go module.

## Example

```go
func main() {
	psfm := poolsuite.NewPoolsuite()
	psfm.Load()
	track := psfm.GetRandomTrackFromPlaylist(psfm.GetRandomPlaylist())
	psfm.Play(track, nil)
	fmt.Println("Waiting ..")
	time.Sleep(8 * time.Second)
	fmt.Println("Stopping ...")
	psfm.PauseResume()
	fmt.Println("Stopped, waiting ...")
	time.Sleep(3 * time.Second)
	track = psfm.GetRandomTrackFromPlaylist(psfm.GetRandomPlaylist())
	psfm.Play(track, nil)
	fmt.Println("Waiting ..")
	time.Sleep(5 * time.Second)
	fmt.Println("Stopping ...")
	psfm.PauseResume()
	fmt.Println("Stopped, waiting ...")
	time.Sleep(3 * time.Second)
	fmt.Println("Done")
}
```

