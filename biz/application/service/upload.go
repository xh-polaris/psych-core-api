package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/google/wire"
	"github.com/xh-polaris/psych-core-api/biz/domain/auth"
	"github.com/xh-polaris/psych-core-api/biz/infra/storage"
	"github.com/xh-polaris/psych-core-api/pkg/errorx"
	"github.com/xh-polaris/psych-core-api/types/enum"
	"github.com/xh-polaris/psych-core-api/types/errno"
)

var _ IFileService = (*FileService)(nil)

type IFileService interface {
	UploadImage(ctx context.Context, fileHeader *multipart.FileHeader, file multipart.File) (*UploadImageResp, error)
	UploadAvatar(ctx context.Context, fileHeader *multipart.FileHeader, file multipart.File) (*UploadImageResp, error)
}

type FileService struct {
	StoragePvd storage.Provider
	AuthDomain auth.IAuthDomain
}

var FileServiceSet = wire.NewSet(
	wire.Struct(new(FileService), "*"),
	wire.Bind(new(IFileService), new(*FileService)),
)

type UploadImageResp struct {
	Code int32  `json:"code"`
	Msg  string `json:"msg"`
	URL  string `json:"url"`
}

var allowedImageExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true, ".bmp": true,
}

func (s *FileService) UploadImage(ctx context.Context, fileHeader *multipart.FileHeader, file multipart.File) (*UploadImageResp, error) {
	meta, err := s.AuthDomain.ExtraUserMeta(ctx)
	if err != nil {
		return nil, err
	}
	if meta.Role < enum.UserRoleUnitAdmin {
		return nil, errorx.New(errno.ErrInsufficientAuth)
	}

	ext := filepath.Ext(fileHeader.Filename)
	if !allowedImageExts[ext] {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "不支持的图片格式，仅支持 jpg/png/gif/webp/bmp"))
	}

	key := fmt.Sprintf("images/%s/%s%s", time.Now().Format("2006/01/02"), uuid.New().String(), ext)
	presignedURL, err := s.StoragePvd.GenPresignUploadURL(ctx, key)
	if err != nil {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "生成预签名上传URL失败"))
	}

	return &UploadImageResp{
		Code: 0,
		Msg:  "success",
		URL:  presignedURL,
	}, nil
}

// UploadAvatar 由后端代传头像，避免学生浏览器直接 PUT COS 时受到存储桶
// CORS 限制。它与管理员使用的预签名上传接口保持隔离。
func (s *FileService) UploadAvatar(ctx context.Context, fileHeader *multipart.FileHeader, file multipart.File) (*UploadImageResp, error) {
	if _, err := s.AuthDomain.ExtraUserMeta(ctx); err != nil {
		return nil, err
	}
	if fileHeader.Size <= 0 || fileHeader.Size > 5*1024*1024 {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "头像大小必须在 5MB 以内"))
	}
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	if !allowedImageExts[ext] {
		return nil, errorx.New(errno.ErrInvalidParams, errorx.KV("field", "不支持的图片格式，仅支持 jpg/png/gif/webp/bmp"))
	}
	key := fmt.Sprintf("avatars/%s/%s%s", time.Now().Format("2006/01/02"), uuid.New().String(), ext)
	accessURL, err := s.StoragePvd.Upload(ctx, key, file, fileHeader.Size)
	if err != nil {
		return nil, errorx.New(errno.ErrInternalError, errorx.KV("field", "头像上传失败"))
	}
	return &UploadImageResp{Code: 0, Msg: "success", URL: accessURL}, nil
}
