package inject

import (
	"fmt"
	"reflect"
)

// Injector 接口集合
type Injector interface {
	Applicator
	Invoker
	TypeMapper
	// 设置父级injector如果当前差找不到参数就往父级查找
	SetParent(Injector)
}

// Applicator 接口
type Applicator interface {
	// 识别struct字段的元数据inject标签，尝试注入对应的字段值, 如果参数interface{}值
	// 不是struct或者struct指针则引发panic，类型注入失败返回错误
	Apply(interface{}) error
}

// Invoker 接口
type Invoker interface {
	// 尝试满足interface{}方法参数并且调用方法，如果interface{}不是方法会引发
	// panic，参数无法满足返回error，正确调用返回原方法的返回值的reflect.Value数组
	Invoke(interface{}) ([]reflect.Value, error)
}

// TypeMapper 注入管理接口
type TypeMapper interface {
	// 把参数interface{}按照reflect.TypeOf返回的类型映射
	Map(interface{}) TypeMapper
	// 把参数1的interface{}映射到参数2的interface{}的接口类型
	// 参数2必须是一个接口指针例如 (*http.ResponseWriter)(nil), 如果按照Map(interface{})的
	// 方式映射的类型就是http.response结构而且不是http.ResponseWriter接口
	MapTo(interface{}, interface{}) TypeMapper
	// 直接设置制定类型映射值
	Set(reflect.Type, reflect.Value) TypeMapper
	// 返回指定类型映射的值，或者返回一个零值可以用v.isValid()检测
	Get(reflect.Type) reflect.Value
}

// injector 内部结构体
type injector struct {
	values map[reflect.Type]reflect.Value
	parent Injector
}

// InterfaceOf 获取一个接口类型通过 (*http.ResponseWriter)(nil) 的方式
func InterfaceOf(value interface{}) reflect.Type {
	t := reflect.TypeOf(value)

	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t.Kind() != reflect.Interface {
		panic("Called inject.InterfaceOf with a value that is not a pointer to an interface. (*MyInterface)(nil)")
	}
	return t
}

// CheckError 检查Invoke反射函数返回的error内容
func CheckError(refvs []reflect.Value) error {
	for _, v := range refvs {
		if v.IsValid() {
			vi := v.Interface()
			switch vi.(type) {
			case error:
				return vi.(error)
			}
		}
	}
	return nil
}

// IsFunction 检查参数是否是方法
func IsFunction(f interface{}) bool {
	v := reflect.ValueOf(f)
	if v.Kind() != reflect.Func {
		return false
	}
	return true
}

// New 创建一个Injector对象
func New() Injector {
	return &injector{
		values: make(map[reflect.Type]reflect.Value),
	}
}

// Invoke 实现Invoker接口
func (inj *injector) Invoke(f interface{}) ([]reflect.Value, error) {
	if !IsFunction(f) {
		panic("f is not kine of reflect.Func")
	}
	t := reflect.TypeOf(f)
	var in = make([]reflect.Value, t.NumIn())

	for i := 0; i < t.NumIn(); i++ {
		argType := t.In(i)
		val := inj.Get(argType)

		if !val.IsValid() {
			return nil, fmt.Errorf("Value not found for type %v", argType)
		}

		in[i] = val
	}
	return reflect.ValueOf(f).Call(in), nil
}

// Apply 实现Applicator接口.
func (inj *injector) Apply(val interface{}) error {
	v := reflect.ValueOf(val)

	for v.Kind() == reflect.Ptr {
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		panic("Value is not representing a struct")
	}

	t := v.Type()

	for i := 0; i < v.NumField(); i++ {
		f := v.Field(i)
		structField := t.Field(i)
		tv, found := structField.Tag.Lookup("inject")
		if f.CanSet() {
			if found {
				ft := f.Type()
				v := inj.Get(ft)

				if !v.IsValid() && tv != "-" {
					return fmt.Errorf("Value not found for type %v", ft)
				}

				f.Set(v)
			}
		}

	}
	return nil
}

// Maps 实现TypeMapper
func (inj *injector) Map(val interface{}) TypeMapper {
	inj.values[reflect.TypeOf(val)] = reflect.ValueOf(val)
	return inj
}

// MapTo 实现TypeMapper
func (inj *injector) MapTo(val interface{}, ifacePtr interface{}) TypeMapper {
	inj.values[InterfaceOf(ifacePtr)] = reflect.ValueOf(val)
	return inj
}

// Set 实现TypeMapper
func (inj *injector) Set(typ reflect.Type, val reflect.Value) TypeMapper {
	inj.values[typ] = val
	return inj
}

// Get 实现TypeMapper
func (inj *injector) Get(t reflect.Type) reflect.Value {
	val := inj.values[t]

	if val.IsValid() {
		return val
	}

	// 没有直接类型匹配，查找接口实现匹配
	if t.Kind() == reflect.Interface {
		for k, v := range inj.values {
			if k.Implements(t) {
				// 这里有一个随机的情况，如果映射的两个实际类型都实现同一个几口，
				// 可能随机返回一个值
				val = v
				break
			}
		}
	}

	// 没有匹配到就直接向上查找
	if !val.IsValid() && inj.parent != nil {
		val = inj.parent.Get(t)
	}

	return val

}

// SetParent 实现Injector接口
func (inj *injector) SetParent(parent Injector) {
	inj.parent = parent
}
