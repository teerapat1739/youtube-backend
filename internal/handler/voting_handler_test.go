package handler

import (
	"strings"
	"testing"

	"be-v2/internal/domain"
)

func TestValidatePersonalInfoRequest(t *testing.T) {
	h := &VotingHandler{}

	tests := []struct {
		name    string
		req     *domain.PersonalInfoRequest
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid request",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลกที่สุด",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "empty first name",
			req: &domain.PersonalInfoRequest{
				FirstName:     "",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "ชื่อจริงต้องมีอย่างน้อย 2 ตัวอักษร",
		},
		{
			name: "first name too short",
			req: &domain.PersonalInfoRequest{
				FirstName:     "ก",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "ชื่อจริงต้องมีอย่างน้อย 2 ตัวอักษร",
		},
		{
			name: "empty last name",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "นามสกุลต้องมีอย่างน้อย 2 ตัวอักษร",
		},
		{
			name: "last name too short",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ข",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "นามสกุลต้องมีอย่างน้อย 2 ตัวอักษร",
		},
		{
			name: "combined name exceeds 255 characters",
			req: &domain.PersonalInfoRequest{
				FirstName:     strings.Repeat("ก", 130),
				LastName:      strings.Repeat("ข", 130),
				Email:         "test@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "ชื่อและนามสกุลรวมกันต้องไม่เกิน 255 ตัวอักษร",
		},
		{
			name: "combined name exactly 255 characters",
			req: &domain.PersonalInfoRequest{
				FirstName:     strings.Repeat("ก", 127),
				LastName:      strings.Repeat("ข", 128),
				Email:         "test@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "empty email",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "กรุณาระบุอีเมลที่ถูกต้อง",
		},
		{
			name: "invalid email without @",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchaiexample.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "กรุณาระบุอีเมลที่ถูกต้อง",
		},
		{
			name: "empty phone",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "หมายเลขโทรศัพท์ต้องมีอย่างน้อย 10 หลัก",
		},
		{
			name: "phone too short",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "08123456",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "หมายเลขโทรศัพท์ต้องมีอย่างน้อย 10 หลัก",
		},
		{
			name: "favorite video exceeds 1000 characters",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: strings.Repeat("ก", 1001),
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "คำตอบต้องไม่เกิน 1000 ตัวอักษร",
		},
		{
			name: "favorite video exactly 1000 characters",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: strings.Repeat("ก", 1000),
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "favorite video with Thai text and emoji",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปที่ตลกมาก 😄 สนุกดี",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "PDPA consent not given",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   false,
			},
			wantErr: true,
			errMsg:  "จำเป็นต้องยอมรับข้อตกลง PDPA เพื่อดำเนินการต่อ",
		},
		{
			name: "names with Thai combining marks",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย์",
				LastName:      "ใจดี๋",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบคลิปตลก",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "names with English characters",
			req: &domain.PersonalInfoRequest{
				FirstName:     "John",
				LastName:      "Doe",
				Email:         "john@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "Great video",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "mixed Thai and English text",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย John",
				LastName:      "Doe ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "ชอบ video ที่ 5",
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
		{
			name: "favorite video with complex Unicode (emoji family)",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: strings.Repeat("👨‍👩‍👧‍👦", 143) + "a", // 143*7 + 1 = 1002 code points (exceeds 1000)
				ConsentPDPA:   true,
			},
			wantErr: true,
			errMsg:  "คำตอบต้องไม่เกิน 1000 ตัวอักษร",
		},
		{
			name: "favorite video with complex Unicode within limit",
			req: &domain.PersonalInfoRequest{
				FirstName:     "สมชาย",
				LastName:      "ใจดี",
				Email:         "somchai@example.com",
				Phone:         "0812345678",
				FavoriteVideo: strings.Repeat("👨‍👩‍👧‍👦", 142), // 142*7 = 994 code points (within 1000)
				ConsentPDPA:   true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := h.validatePersonalInfoRequest(tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePersonalInfoRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validatePersonalInfoRequest() error message = %v, want containing %v", err.Error(), tt.errMsg)
			}
		})
	}
}

// Test Unicode character counting
func TestUnicodeCharacterCounting(t *testing.T) {
	h := &VotingHandler{}

	tests := []struct {
		name      string
		firstName string
		lastName  string
		wantErr   bool
		errMsg    string
	}{
		{
			name:      "Thai text with tone marks",
			firstName: "สมชาย์",     // 6 characters (including tone mark)
			lastName:  "ใจดี่ครั้บ", // 9 characters
			wantErr:   false,
		},
		{
			name:      "exactly 255 total characters",
			firstName: strings.Repeat("ก", 100),
			lastName:  strings.Repeat("ข", 155),
			wantErr:   false,
		},
		{
			name:      "256 total characters",
			firstName: strings.Repeat("ก", 100),
			lastName:  strings.Repeat("ข", 156),
			wantErr:   true,
			errMsg:    "ชื่อและนามสกุลรวมกันต้องไม่เกิน 255 ตัวอักษร",
		},
		{
			name:      "multi-byte Thai characters",
			firstName: strings.Repeat("ฟ", 128), // Thai character (3 bytes in UTF-8)
			lastName:  strings.Repeat("ห", 128), // Total: 256 characters
			wantErr:   true,
			errMsg:    "ชื่อและนามสกุลรวมกันต้องไม่เกิน 255 ตัวอักษร",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := &domain.PersonalInfoRequest{
				FirstName:     tt.firstName,
				LastName:      tt.lastName,
				Email:         "test@example.com",
				Phone:         "0812345678",
				FavoriteVideo: "test",
				ConsentPDPA:   true,
			}
			err := h.validatePersonalInfoRequest(req)
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePersonalInfoRequest() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
				t.Errorf("validatePersonalInfoRequest() error message = %v, want containing %v", err.Error(), tt.errMsg)
			}
		})
	}
}