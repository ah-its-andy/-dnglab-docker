package repo

import (
	"crypto/sha256"
	"encoding/base64"
	"errors"

	"gorm.io/gorm"
)

type FileIndexModel struct {
	gorm.Model

	FileName     string `gorm:"column:file_name;type:text;size:65535"`
	FileNameHash string `gorm:"column:file_name_hash;type:varchar;size:512"`
}

func (FileIndexModel) TableName() string {
	return "file_index"
}

func Create(gdb *gorm.DB, model *FileIndexModel) error {
	return gdb.Table(model.TableName()).Create(model).Error
}

func FindByName(gdb *gorm.DB, name string) (*FileIndexModel, error) {
	var model FileIndexModel
	err := gdb.Table(model.TableName()).Where(map[string]any{
		"file_name_hash": base64.StdEncoding.EncodeToString(sha256.New().Sum([]byte(name))),
	}).Find(&model).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, err
	}
	if model.ID == 0 {
		return nil, nil
	}
	return &model, nil
}
