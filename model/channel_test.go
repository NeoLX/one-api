package model

import (
	"testing"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupChannelTestDB(t *testing.T) {
	t.Helper()
	var err error
	DB, err = gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err = DB.AutoMigrate(&Channel{}, &Ability{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	if err := DB.Exec("DELETE FROM channels").Error; err != nil {
		t.Fatalf("清空 channels 表失败: %v", err)
	}
}

func TestGetAllChannelsOmitsKeyAndConfig(t *testing.T) {
	setupChannelTestDB(t)

	ch := &Channel{
		Type:   1,
		Key:    "sk-secret",
		Name:   "test-channel",
		Status: ChannelStatusEnabled,
		Config: `{"headers":{"X-Title":"my-gateway"}}`,
	}
	if err := DB.Create(ch).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}

	channels, err := GetAllChannels(0, 10, "limited")
	if err != nil {
		t.Fatalf("GetAllChannels 失败: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("期望 1 个渠道，实际 %d", len(channels))
	}
	if channels[0].Key != "" {
		t.Errorf("列表接口不应返回 key，实际 %q", channels[0].Key)
	}
	if channels[0].Config != "" {
		t.Errorf("列表接口不应返回 config，实际 %q", channels[0].Config)
	}

	// 单条查询必须仍然返回 config：前端编辑页要靠它回填
	one, err := GetChannelById(ch.Id, false)
	if err != nil {
		t.Fatalf("GetChannelById 失败: %v", err)
	}
	if one.Config == "" {
		t.Errorf("GetChannelById 必须返回 config")
	}
	if one.Key != "" {
		t.Errorf("GetChannelById(selectAll=false) 不应返回 key，实际 %q", one.Key)
	}
}
