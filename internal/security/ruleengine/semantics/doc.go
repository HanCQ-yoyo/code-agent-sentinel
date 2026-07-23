// Package semantics 实现危险命令的语义级解析(Go 重写 dcg 语义解析器)。
// 与纯正则规则正交:语义解析器对结构化命令(argv 可解析的)做精确判断,
// 解决正则无法区分「执行区 vs 数据区」「flag 拆分」「alias 展开」的误报/漏报。
// RulesDetector 在正则求值前调用语义解析器:Deny→直接报,Safe→跳过该命令,Unknown→走正则。
package semantics
