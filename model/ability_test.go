package model

import (
	"testing"

	"github.com/songquanpeng/one-api/common"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyChannelTestDB(t *testing.T) {
	t.Helper()
	// GetRandomSatisfiedChannel 会按 UsingSQLite 决定用 RANDOM() 还是 RAND()
	originalUsingSQLite := common.UsingSQLite
	common.UsingSQLite = true
	t.Cleanup(func() { common.UsingSQLite = originalUsingSQLite })

	var err error
	DB, err = gorm.Open(sqlite.Open("file::proxyfilter?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if err = DB.AutoMigrate(&Channel{}, &Ability{}); err != nil {
		t.Fatalf("建表失败: %v", err)
	}
	for _, table := range []string{"channels", "abilities"} {
		if err := DB.Exec("DELETE FROM " + table).Error; err != nil {
			t.Fatalf("清空 %s 失败: %v", table, err)
		}
	}
}

func mustCreateChannel(t *testing.T, channelType int, models string) *Channel {
	t.Helper()
	channel := &Channel{
		Type:     channelType,
		Key:      "sk-test",
		Name:     "channel",
		Status:   ChannelStatusEnabled,
		Models:   models,
		Group:    "default",
		Priority: func() *int64 { p := int64(0); return &p }(),
	}
	if err := DB.Create(channel).Error; err != nil {
		t.Fatalf("插入渠道失败: %v", err)
	}
	if err := channel.AddAbilities(); err != nil {
		t.Fatalf("写入 abilities 失败: %v", err)
	}
	return channel
}

// allowedTypes 为 nil 时必须与加参数前一致：能选到渠道（回归）
func TestGetRandomSatisfiedChannelWithoutAllowedTypes(t *testing.T) {
	setupProxyChannelTestDB(t)
	mustCreateChannel(t, 1, "dup-model")

	for i := 0; i < 5; i++ {
		channel, err := GetRandomSatisfiedChannel("default", "dup-model", false, nil)
		if err != nil {
			t.Fatalf("不限制时应当能选到渠道: %v", err)
		}
		if channel.Type != 1 {
			t.Errorf("渠道类型不对: got %d", channel.Type)
		}
	}
}

// 同名模型挂在两个不同类型的渠道上时，只允许其中一个供应商，就必须只选到它
func TestGetRandomSatisfiedChannelFiltersByAllowedTypes(t *testing.T) {
	setupProxyChannelTestDB(t)
	openai := mustCreateChannel(t, 1, "dup-model")
	custom := mustCreateChannel(t, 8, "dup-model")

	tests := []struct {
		name         string
		allowedTypes map[int]bool
		wantChannel  int
	}{
		{name: "只允许 OpenAI", allowedTypes: map[int]bool{1: true}, wantChannel: openai.Id},
		{name: "只允许 Custom", allowedTypes: map[int]bool{8: true}, wantChannel: custom.Id},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for i := 0; i < 20; i++ {
				channel, err := GetRandomSatisfiedChannel("default", "dup-model", false, tt.allowedTypes)
				if err != nil {
					t.Fatalf("应当能选到渠道: %v", err)
				}
				if channel.Id != tt.wantChannel {
					t.Fatalf("选到了不该选的渠道 #%d（类型 %d）", channel.Id, channel.Type)
				}
			}
		})
	}
}

func TestGetRandomSatisfiedChannelWithEmptyAllowedTypesSet(t *testing.T) {
	setupProxyChannelTestDB(t)
	mustCreateChannel(t, 1, "dup-model")

	// 空 map 表示"没有任何供应商被允许"：不能选到任何渠道
	if _, err := GetRandomSatisfiedChannel("default", "dup-model", false, map[int]bool{}); err == nil {
		t.Fatal("空允许集合不应选到渠道")
	}
}

func TestGetRandomSatisfiedChannelAllowedTypeNotInDB(t *testing.T) {
	setupProxyChannelTestDB(t)
	mustCreateChannel(t, 1, "dup-model")

	if _, err := GetRandomSatisfiedChannel("default", "dup-model", false, map[int]bool{33: true}); err == nil {
		t.Fatal("允许的类型在库里没有渠道时应当报错")
	}
}

// allowedTypes 里含该类型但该模型没挂在这个类型上时，不能误选
func TestGetRandomSatisfiedChannelModelOnlyOnOtherType(t *testing.T) {
	setupProxyChannelTestDB(t)
	mustCreateChannel(t, 1, "model-a")
	mustCreateChannel(t, 8, "model-b")

	if _, err := GetRandomSatisfiedChannel("default", "model-a", false, map[int]bool{8: true}); err == nil {
		t.Fatal("model-a 只挂在类型 1 上，只允许类型 8 时不应选到渠道")
	}
}
