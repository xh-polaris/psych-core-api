package service

import (
	"context"
	"fmt"
	"mime/multipart"
	"path/filepath"
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
