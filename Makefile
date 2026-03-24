.PHONY: start build wire update new clean convert-annotations restore-annotations fix-imports

SERVICE_NAME := psych.core_api
MODULE_NAME := github.com/xh-polaris/psych-core-api

HANDLER_DIR := biz/adaptor/controller
MODEL_DIR := biz/application/dto
ROUTER_DIR := biz/adaptor/router

IDL_DIR ?= ../psych-idl
# 提取最后一个部分（如 "psych.core_api" → "core_api"）
MAIN_IDL_BASE := $(subst -,_,$(shell echo $(SERVICE_NAME) | awk -F '.' '{print $$NF}'))
# 组合成最终路径（如 "$(IDL_DIR)/core_api/core_api.proto"）
FULL_MAIN_IDL_PATH := $(IDL_DIR)/$(MAIN_IDL_BASE)/$(MAIN_IDL_BASE).proto
# 这里由于 idl 中未直接放置 google 相关文件，所以必须把这个文件放置到下面这个文件夹中
IDL_OPTIONS := -I $(IDL_DIR) --idl $(FULL_MAIN_IDL_PATH)
OUTPUT_OPTIONS := --handler_dir $(HANDLER_DIR) --model_dir $(MODEL_DIR) --router_dir $(ROUTER_DIR)
EXTRA_OPTIONS := --pb_camel_json_tag=true --unset_omitempty=true

run:
	sh ./output/bootstrap.sh
build:
	sh ./build.sh
build_and_run:
	sh ./build.sh && sh ./output/bootstrap.sh
wire:
	wire ./provider
convert-annotations:
	@echo "Converting google.api.http annotations to hz annotations..."
	@$(IDL_DIR)/scripts/convert_to_hz_annotations.sh $(FULL_MAIN_IDL_PATH)
restore-annotations:
	@echo "Restoring original proto file from git..."
	@cd $(IDL_DIR) && git checkout -- $(MAIN_IDL_BASE)/$(MAIN_IDL_BASE).proto 2>/dev/null || echo "Warning: Could not restore proto file from git"
fix-imports:
	@echo "Fixing invalid imports..."
	@# 删除所有对 http 的导入（http.proto 只是注解定义，不需要在生成的代码中引用）
	@find $(MODEL_DIR) -name "*.pb.go" -type f -exec sed -i '' '/_ "github.com\/xh-polaris\/psych-core-api\/biz\/application\/dto\/http"/d' {} \; 2>/dev/null || \
		find $(MODEL_DIR) -name "*.pb.go" -type f -exec sed -i '/_ "github.com\/xh-polaris\/psych-core-api\/biz\/application\/dto\/http"/d' {} \; 2>/dev/null || true
	@# 删除不必要的 http 目录（http.proto 只是注解定义，不需要生成代码）
	@rm -rf $(MODEL_DIR)/http
	@# 删除 google.golang.org 目录（这是外部依赖，不需要生成）
	@rm -rf $(MODEL_DIR)/google.golang.org
update: convert-annotations
	hz --verbose update $(IDL_OPTIONS) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
	@files=$$(find biz/application/dto -type f); \
	for file in $$files; do \
  	  sed -i  -e 's/func init\(\).*//' $$file; \
  	done
	@make fix-imports
	@make restore-annotations
update-macos: convert-annotations
	hz --verbose update $(IDL_OPTIONS) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
	@find biz/application/dto -name "*.go" | xargs perl -i -pe 's/func init\(\).*//'
	@make fix-imports
	@make restore-annotations
new:
	hz new $(IDL_OPTIONS) $(OUTPUT_OPTIONS) --service $(SERVICE_NAME) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
clean:
	rm -r ./output
swag:
	swag init -g main.go --parseDependency --parseInternal
idl:
	go get github.com/xh-polaris/psych-idl@main
