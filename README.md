浏览器更新服务器svn
===
访问浏览器，svn查看及更新指定代码库，用于不在本地运行且修改的文件不需要重启服务的项目（其实可以直接使用svn hook实现）


## 项目文件结构说明

```
.
├── README.md
├── config.json 配置文件
├── config.sample.json
├── main.go 后端代码
├── static 静态文件
│   ├── css
│   ├── img
│   └── js
│       └── lib
│           ├── jquery-1.7.1.min.js
│           ├── zepto.js
│           └── zepto.min.js
└── views 页面
    ├── base.html
    └── home.html

```