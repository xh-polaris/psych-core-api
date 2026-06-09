.PHONY: start build wire update new clean convert-annotations restore-annotations fix-imports

SERVICE_NAME := psych.core_api
MODULE_NAME := github.com/xh-polaris/psych-core-api

HANDLER_DIR := biz/adaptor/controller
MODEL_DIR := biz/application/dto
ROUTER_DIR := biz/adaptor/router

IDL_DIR ?= ../psych-idl
MAIN_IDL_BASE := $(subst -,_,$(shell echo $(SERVICE_NAME) | awk -F '.' '{print $$NF}'))
FULL_MAIN_IDL_PATH := $(IDL_DIR)/$(MAIN_IDL_BASE)/$(MAIN_IDL_BASE).proto
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
	@# 删除 dto/*.pb.go 中不存在的 http 包引用
	@sed -i.bak '\#_ "github.com/xh-polaris/psych-core-api/biz/application/dto/http"#d' $(MODEL_DIR)/*/*.pb.go $(MODEL_DIR)/*.pb.go 2>/dev/null || true
	@find $(MODEL_DIR) -name "*.pb.go.bak" -delete 2>/dev/null || true
	@# 删除 controller 中未使用的 basic 包引用
	@for f in $(HANDLER_DIR)/core_api/*.go $(HANDLER_DIR)/*.go; do \
		[ -f "$$f" ] || continue; \
		grep -q 'basic\.' "$$f" 2>/dev/null && continue; \
		sed -i.bak '/basic "github.com\/xh-polaris\/psych-core-api\/biz\/application\/dto\/basic"/d' "$$f" 2>/dev/null || true; \
	done
	@find $(HANDLER_DIR) -name "*.go.bak" -delete 2>/dev/null || true
	@# 删除不必要的生成目录
	@rm -rf $(MODEL_DIR)/http
	@rm -rf $(MODEL_DIR)/google.golang.org
update: convert-annotations
	hz --verbose update $(IDL_OPTIONS) --mod $(MODULE_NAME) $(EXTRA_OPTIONS)
	@# 删除 dto 中自动生成的 init 函数
	@sed -i.bak 's/func init().*//' $$(find $(MODEL_DIR) -name "*.pb.go" -type f) 2>/dev/null || true
	@find $(MODEL_DIR) -name "*.pb.go.bak" -delete 2>/dev/null || true
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
