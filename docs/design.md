# 规范要求
- 对于外部接口，需要采用动词+名词的方式。比如CreateFile

# 技术栈
- http框架采用 gin
- studio-server 需要支持优雅关闭
- websocket 采用 coder/websocket 库
- 数据库采用 mysql，采用gorm
- 日志框架采用 zap
- 配置文件采用yaml，通过viper读取
- 需要采用整洁架构来组织代码目录

