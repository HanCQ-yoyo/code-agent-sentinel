// Monaco worker 注册(Vite 官方 ?worker 模式)。
// 只引 editor.worker(主进程通用)+ json.worker(JSON 语法专用)。
// markdown/shell/python 等语言用 Monaco 内置 Monarch 分词器在主线程跑,无需专用 worker。
// 此模块为副作用模块:被 MonacoViewer import 时触发 self.MonacoEnvironment 注册。
/// <reference types="vite/client" />
import type {} from 'monaco-editor'
import editorWorker from 'monaco-editor/esm/vs/editor/editor.worker?worker'
import jsonWorker from 'monaco-editor/esm/vs/language/json/json.worker?worker'

self.MonacoEnvironment = {
  getWorker(_workerId: string, label: string) {
    if (label === 'json') return new jsonWorker()
    return new editorWorker()
  },
}
