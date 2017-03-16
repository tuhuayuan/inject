package inject

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

type SpecialString interface {
}

type TestStruct struct {
	Dep1 string        `inject:"required" json:"dep1"`
	Dep2 SpecialString `inject:"-"`

	// 重复Dep1类型, 无法实现应该像Dep2那样自定义一个类型
	Dep3 string
}

type Greeter struct {
	Name string
}

func (g *Greeter) String() string {
	return "Hello, My name is" + g.Name
}

func Test_InjectorInvoke(t *testing.T) {
	inj := New()
	assert.NotNil(t, inj)

	// 映射字符串
	dep1 := "some dependency"
	inj.Map(dep1)

	// 映射接口
	dep2 := "another dep"
	inj.MapTo(dep2, (*SpecialString)(nil))

	// 映射有方向的通道
	dep3 := make(chan *SpecialString)
	dep4 := make(chan *SpecialString)
	typRecv := reflect.ChanOf(reflect.RecvDir, reflect.TypeOf(dep3).Elem())
	typSend := reflect.ChanOf(reflect.SendDir, reflect.TypeOf(dep4).Elem())
	inj.Set(typRecv, reflect.ValueOf(dep3))
	inj.Set(typSend, reflect.ValueOf(dep4))

	// 映射双向通道
	dep5 := make(chan *SpecialString)
	inj.Map(dep5)

	rvs, err := inj.Invoke(
		func(d1 string, d2 SpecialString, d3 <-chan *SpecialString, d4 chan<- *SpecialString, d5 chan *SpecialString) {
			assert.Equal(t, d1, dep1)
			assert.Equal(t, d2, dep2)
			assert.Equal(t, reflect.TypeOf(d3).Elem(), reflect.TypeOf(dep3).Elem())
			assert.Equal(t, reflect.TypeOf(d4).Elem(), reflect.TypeOf(dep4).Elem())
			assert.Equal(t, reflect.TypeOf(d3).ChanDir(), reflect.RecvDir)
			assert.Equal(t, reflect.TypeOf(d4).ChanDir(), reflect.SendDir)
		})
	assert.NoError(t, err)
	assert.NoError(t, CheckError(rvs))
}

func Test_InjectorInvokeReturnValues(t *testing.T) {
	inj := New()
	assert.NotNil(t, inj)

	rvs, err := inj.Invoke(func() (string, bool) {
		return "test", false
	})

	assert.NoError(t, err)
	assert.Len(t, rvs, 2)
	assert.Equal(t, "test", rvs[0].String())
	assert.Equal(t, false, rvs[1].Bool())
}

func Test_InjectorApply(t *testing.T) {
	inj := New()
	assert.NotNil(t, inj)

	inj.Map("dep1").MapTo("dep2", (*SpecialString)(nil))

	s1 := TestStruct{}
	err := inj.Apply(&s1)
	assert.NoError(t, err)

	assert.Equal(t, s1.Dep1, "dep1")
	assert.Equal(t, s1.Dep2, "dep2")
	assert.Equal(t, s1.Dep3, "")

	inj = New()
	assert.NotNil(t, inj)
	inj.MapTo("dep2", (*SpecialString)(nil))

	s2 := TestStruct{}
	err = inj.Apply(&s2)
	assert.Error(t, err)
}

func Test_InterfaceOf(t *testing.T) {
	iType := InterfaceOf((*SpecialString)(nil))
	assert.Equal(t, iType.Kind(), reflect.Interface)

	iType = InterfaceOf((**SpecialString)(nil))
	assert.Equal(t, iType.Kind(), reflect.Interface)

	// Expecting nil
	defer func() {
		rec := recover()
		assert.NotNil(t, rec)
	}()
	iType = InterfaceOf((*testing.T)(nil))
}

func Test_InjectorSetParent(t *testing.T) {
	injParent := New()
	injParent.MapTo("another dep", (*SpecialString)(nil))

	inj := New()
	inj.SetParent(injParent)

	assert.Equal(t, inj.Get(InterfaceOf((*SpecialString)(nil))).IsValid(), true)
}

func Test_InjectImplementors(t *testing.T) {
	inj := New()
	g := &Greeter{"Jeremy"}
	inj.Map(g)

	assert.Equal(t, inj.Get(InterfaceOf((*fmt.Stringer)(nil))).IsValid(), true)
}
