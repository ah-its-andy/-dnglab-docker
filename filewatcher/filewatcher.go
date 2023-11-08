package filewatcher

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type FileInfo struct {
	Name      string
	LastWrite time.Time
}

type FileWatcher struct {
	directories  []string
	subscribers  []chan string
	stopChannels []chan bool
	fileInfoMap  *sync.Map
	writeTimeout time.Duration
}

func New(directories ...string) *FileWatcher {
	return &FileWatcher{
		directories:  directories,
		subscribers:  make([]chan string, 0),
		stopChannels: make([]chan bool, 0),
		fileInfoMap:  &sync.Map{},
		writeTimeout: time.Second * 15,
	}
}
func (fw *FileWatcher) Watch() {
	fw.walk()
	fw.startWatch()
}

func (fw *FileWatcher) walk() {
	for _, directoryPath := range fw.directories {
		err := filepath.Walk(directoryPath, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() {
				fileInfo := &FileInfo{
					Name:      path,
					LastWrite: time.Now().UTC(),
				}
				fw.fileInfoMap.Store(path, fileInfo)
			}

			return nil
		})

		if err != nil {
			log.Fatal(err)
		}
	}
}

func (fw *FileWatcher) startWatch() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	go fw.listenWatcher(watcher)
	go fw.scanFileInfoMap()
	for _, directory := range fw.directories {
		stop := make(chan bool)
		fw.stopChannels = append(fw.stopChannels, stop)
		watcher.Add(directory)
	}

}

func (fw *FileWatcher) listenWatcher(watcher *fsnotify.Watcher) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}

			// 监控到的事件类型
			if event.Op&fsnotify.Create == fsnotify.Create || event.Op&fsnotify.Write == fsnotify.Write {
				fileInfo, ok := fw.fileInfoMap.Load(event.Name)
				if !ok {
					fileInfo = &FileInfo{
						Name:      event.Name,
						LastWrite: time.Now().UTC(),
					}
					fw.fileInfoMap.Store(event.Name, fileInfo)
				} else {
					fileInfo.(*FileInfo).LastWrite = time.Now().UTC()
				}
			}

		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.Println("Error:", err)
		}
	}
}

func (fw *FileWatcher) scanFileInfoMap() {
	for {
		fileName := ""
		fw.fileInfoMap.Range(func(key, value any) bool {
			fileInfo := value.(*FileInfo)
			if time.Now().UTC().Sub(fileInfo.LastWrite).Microseconds() > fw.writeTimeout.Microseconds() {
				fileName = key.(string)
				return false
			}
			return true
		})
		if len(fileName) > 0 {
			fw.fileInfoMap.Delete(fileName)
			for _, subscriber := range fw.subscribers {
				subscriber <- fileName
			}
		} else {
			time.Sleep(time.Second * 15)
		}
	}
}

func (fw *FileWatcher) Subscribe() chan string {
	subscriber := make(chan string)
	fw.subscribers = append(fw.subscribers, subscriber)
	return subscriber
}

func (fw *FileWatcher) Stop() {
	for _, stop := range fw.stopChannels {
		stop <- true
	}
}
