package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	//"gorm.io/driver/sqlite" // Sqlite driver based on CGO
	"github.com/ah-its-andy/dnglab-docker/filewatcher"
	"github.com/ah-its-andy/dnglab-docker/repo"
	"github.com/bwmarrin/snowflake"
	"github.com/glebarez/sqlite" // Pure go SQLite driver, checkout https://github.com/glebarez/sqlite for details
	"gorm.io/gorm"
)

var (
	DataDir   string
	SourceDir string
	DestDir   string
	FileExts  []string
	DBConn    *gorm.DB
	snowGen   *snowflake.Node
)

func main() {
	DataDir = os.Getenv("DATA_DIR")
	SourceDir = os.Getenv("SOURCE_DIR")
	DestDir = os.Getenv("DEST_DIR")
	FileExts = strings.Split(os.Getenv("FILE_EXTS"), ",")

	dbfile := filepath.Join(DataDir, "indexdb.db")
	var err error
	snowGen, err = snowflake.NewNode(1)
	if err != nil {
		log.Fatalf("init snowflake node failed: %v", err)
	}

	var initDB bool
	if _, err := os.Stat(dbfile); errors.Is(err, os.ErrNotExist) {
		initDB = true
		f, err := os.Create(dbfile)
		if err != nil {
			log.Fatalf("create database file failed: %v", err)
		}
		f.Close()
	}

	DBConn, err = gorm.Open(sqlite.Open(dbfile), &gorm.Config{})
	if err != nil {
		log.Fatalf("open database file '%s' failed: %v", dbfile, err)
	}

	if initDB {
		if err := DBConn.AutoMigrate(&repo.FileIndexModel{}); err != nil {
			log.Fatalf("error migrating database: %v", err)
		}
	}

	watcher := filewatcher.New(SourceDir)
	subc := watcher.Subscribe()
	go watcher.Watch()

	for {
		file := <-subc
		log.Printf("File Changed: %v \r\n", file)
		fileExt := filepath.Ext(file)
		fileExtMatch := false
		for _, ext := range FileExts {
			if strings.EqualFold(fileExt, ext) {
				fileExtMatch = true
				break
			}
		}
		if !fileExtMatch {
			log.Printf("%s | File extension not supported.\r\n", filepath.Base(file))
			continue
		}
		err := DBConn.Transaction(func(tx *gorm.DB) error {
			fileModel, err := repo.FindByName(tx, file)
			if err != nil {
				return fmt.Errorf("find file index error: %v", err)
			}
			if fileModel != nil {
				log.Printf("%s | Skipped \r\n", filepath.Base(file))
				return nil
			}
			fileModel = &repo.FileIndexModel{}
			fileModel.ID = uint(snowGen.Generate().Int64())
			fileModel.FileName = file
			fileModel.FileNameHash = base64.StdEncoding.EncodeToString(sha256.New().Sum([]byte(file)))
			fileModel.CreatedAt = time.Now().UTC()
			if err := repo.Create(tx, fileModel); err != nil {
				return fmt.Errorf("error creating file index: %v", err)
			}

			fileName := filepath.Base(file)
			destFile := filepath.Join(DestDir, fileName)
			if err := ConvertFile(file, destFile); err != nil {
				return fmt.Errorf("error converting file: %v", err)
			}
			return nil
		})
		if err != nil {
			log.Fatalf("%s | %v", filepath.Base(file), err)
		}
	}
}

func ConvertFile(sourceFile string, destFile string) error {
	//dnglab convert IMG_1234.CR3 IMG_1234.DNG
	cmd := exec.Command("dnglab", "-d", "-v", "convert", sourceFile, destFile) // 替换成你要执行的外部命令和参数

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %v", err)
	}

	log.Printf("%s | dnglab convert begins \r\n", filepath.Base(sourceFile))
	cmd.Start()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Printf("%s | %s \r\n", filepath.Base(sourceFile), scanner.Text())
	}

	cmd.Wait()

	return nil
}
