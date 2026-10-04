/*
 * Copyright (C) 2026 Franklin D. Amador
 *
 * This software is dual-licensed under the terms of the GPL v2.0 and
 * a commercial license. You may choose to use this software under either
 * license.
 *
 * See the LICENSE files in the project root for full license text.
 */

package plcopen

import (
	"encoding/xml"
	"strings"
	"testing"
)

// A project from another tool, with additional data on every kind of
// element, in a namespace the root declares (cds) and in no namespace.
const toolProject = `<?xml version="1.0" encoding="utf-8"?>
<project xmlns="http://www.plcopen.org/xml/tc6_0201" xmlns:xhtml="http://www.w3.org/1999/xhtml" xmlns:cds="http://www.3s-software.com/plcopenxml">
  <fileHeader companyName="Tool" productName="Tool" productVersion="1" creationDateTime="2026-10-04T12:00:00"/>
  <contentHeader name="Plant">
    <coordinateInfo><fbd><scaling x="1" y="1"/></fbd><ld><scaling x="1" y="1"/></ld><sfc><scaling x="1" y="1"/></sfc></coordinateInfo>
    <addDataInfo><info name="http://example.com/hmi" version="1.0" vendor="http://example.com"/></addDataInfo>
    <addData><data name="http://example.com/header" handleUnknown="preserve"><cds:Setting value="1"/></data></addData>
  </contentHeader>
  <types>
    <dataTypes>
      <dataType name="Valve">
        <baseType><struct>
          <variable name="Open"><type><BOOL/></type>
            <addData><data name="http://example.com/hmi" handleUnknown="preserve"><Label>Open</Label></data></addData>
          </variable>
        </struct></baseType>
        <addData><data name="http://example.com/faceplate" handleUnknown="preserve"><Faceplate kind="valve"/></data></addData>
      </dataType>
    </dataTypes>
    <pous>
      <pou name="Tank" pouType="program">
        <interface>
          <localVars>
            <variable name="Level"><type><REAL/></type>
              <addData>
                <data name="http://example.com/hmi" handleUnknown="preserve"><hmi:tag xmlns:hmi="http://example.com/hmi" unit="m" format="%.2f"/></data>
                <data name="http://example.com/cache" handleUnknown="discard"><Cache>stale</Cache></data>
              </addData>
            </variable>
            <variable name="Run"><type><BOOL/></type></variable>
            <addData><data name="http://example.com/list" handleUnknown="implementation"><Folder>Tank</Folder></data></addData>
          </localVars>
          <addData><data name="http://example.com/interface" handleUnknown="preserve"><Note/></data></addData>
        </interface>
        <body>
          <ST><xhtml:p>Run := Level &lt; 10.0;</xhtml:p></ST>
          <addData><data name="http://example.com/body" handleUnknown="preserve"><cds:Folding/></data></addData>
        </body>
        <addData><data name="http://example.com/pou" handleUnknown="preserve"><cds:ObjectId>42</cds:ObjectId></data></addData>
      </pou>
    </pous>
  </types>
  <instances>
    <configurations>
      <configuration name="Cfg">
        <resource name="Res">
          <task name="Fast" priority="1" interval="T#10ms">
            <pouInstance name="T1" typeName="Tank"/>
            <addData><data name="http://example.com/task" handleUnknown="preserve"><Watchdog ms="50"/></data></addData>
          </task>
          <addData><data name="http://example.com/resource" handleUnknown="preserve"><Cpu/></data></addData>
        </resource>
        <addData><data name="http://example.com/config" handleUnknown="preserve"><Device/></data></addData>
      </configuration>
    </configurations>
  </instances>
  <addData><data name="http://example.com/project" handleUnknown="preserve"><cds:ProjectStructure/></data></addData>
</project>`

// hmiTag is the content of the example HMI additional data.
type hmiTag struct {
	XMLName xml.Name `xml:"http://example.com/hmi tag"`
	Unit    string   `xml:"unit,attr"`
	Format  string   `xml:"format,attr"`
}

// Reading a project keeps the additional data of each element, and writing
// it back writes it again, with its namespaces, valid without the prefixes
// the root declared.
func TestAddDataSurvivesImportAndMarshal(t *testing.T) {
	proj, err := Import([]byte(toolProject))
	if err != nil {
		t.Fatal(err)
	}
	pou := proj.Types.Pous.Pous[0]
	level := pou.Interface.LocalVars[0].Variables[0]
	checks := map[string]*AddData{
		"project":   proj.AddData,
		"header":    proj.ContentHeader.AddData,
		"dataType":  proj.Types.DataTypes.DataTypes[0].AddData,
		"member":    proj.Types.DataTypes.DataTypes[0].BaseType.Struct.Variables[0].AddData,
		"pou":       pou.AddData,
		"interface": pou.Interface.AddData,
		"varList":   pou.Interface.LocalVars[0].AddData,
		"variable":  level.AddData,
		"body":      pou.Body.AddData,
		"config":    proj.Instances.Configurations.Configurations[0].AddData,
		"resource":  proj.Instances.Configurations.Configurations[0].Resources[0].AddData,
		"task":      proj.Instances.Configurations.Configurations[0].Resources[0].Tasks[0].AddData,
	}
	for where, a := range checks {
		if a == nil || len(a.Data) == 0 {
			t.Errorf("%s: no additional data", where)
		}
	}
	if info := proj.ContentHeader.AddDataInfo; info == nil || len(info.Info) != 1 || info.Info[0].Vendor != "http://example.com" {
		t.Errorf("addDataInfo: %+v", proj.ContentHeader.AddDataInfo)
	}

	out, err := marshalProject(proj)
	if err != nil {
		t.Fatal(err)
	}
	again, err := Import(out)
	if err != nil {
		t.Fatalf("the written project does not read back: %v\n%s", err, out)
	}
	var tag hmiTag
	if err := again.Types.Pous.Pous[0].Interface.LocalVars[0].Variables[0].AddData.Get("http://example.com/hmi").Decode(&tag); err != nil {
		t.Fatal(err)
	}
	if tag.Unit != "m" || tag.Format != "%.2f" {
		t.Errorf("the HMI tag came back as %+v", tag)
	}
	var id struct {
		XMLName xml.Name `xml:"http://www.3s-software.com/plcopenxml ObjectId"`
		Value   string   `xml:",chardata"`
	}
	if err := again.Types.Pous.Pous[0].AddData.Get("http://example.com/pou").Decode(&id); err != nil || id.Value != "42" {
		t.Errorf("an element in a namespace the root declared: %+v %v", id, err)
	}
	if !strings.Contains(string(out), `handleUnknown="discard"`) {
		t.Error("written as read, the discard entry is kept")
	}
}

// Editing an imported project's code and exporting it keeps the additional
// data of the elements that are still there, except entries to discard.
func TestExportSourceKeepingAddData(t *testing.T) {
	proj, err := Import([]byte(toolProject))
	if err != nil {
		t.Fatal(err)
	}
	text, err := ImportToIECText([]byte(toolProject))
	if err != nil {
		t.Fatal(err)
	}
	edited := strings.Replace(text, "10.0", "12.5", 1)
	out, err := ExportSourceKeeping(edited, "Plant", proj)
	if err != nil {
		t.Fatalf("%v\n%s", err, edited)
	}
	pou := out.Types.Pous.Pous[0]
	level := pou.Interface.LocalVars[0].Variables[0]
	if level.AddData.Get("http://example.com/hmi") == nil {
		t.Error("the variable's HMI data was lost")
	}
	if level.AddData.Get("http://example.com/cache") != nil {
		t.Error("the variable's discard entry was kept")
	}
	for where, a := range map[string]*AddData{
		"project": out.AddData, "header": out.ContentHeader.AddData, "pou": pou.AddData, "body": pou.Body.AddData,
		"interface": pou.Interface.AddData, "varList": pou.Interface.LocalVars[0].AddData,
		"dataType": out.Types.DataTypes.DataTypes[0].AddData,
		"member":   out.Types.DataTypes.DataTypes[0].BaseType.Struct.Variables[0].AddData,
		"config":   out.Instances.Configurations.Configurations[0].AddData,
		"resource": out.Instances.Configurations.Configurations[0].Resources[0].AddData,
		"task":     out.Instances.Configurations.Configurations[0].Resources[0].Tasks[0].AddData,
	} {
		if a == nil || len(a.Data) == 0 {
			t.Errorf("%s: additional data lost", where)
		}
	}
	if out.ContentHeader.AddDataInfo == nil {
		t.Error("addDataInfo lost")
	}
	if !strings.Contains(out.Types.Pous.Pous[0].Body.ST.Text, "12.5") {
		t.Error("the edit is not in the export")
	}
	KeepAddData(nil, out) // nothing to carry
}

// A program reads and writes its own additional data by name.
func TestAddDataGetSet(t *testing.T) {
	var a *AddData
	if a.Get("x") != nil {
		t.Error("Get on nil")
	}
	a, err := a.Set("http://example.com/hmi", HandlePreserve, hmiTag{Unit: "bar", Format: "%.1f"})
	if err != nil {
		t.Fatal(err)
	}
	a, _ = a.Set("http://example.com/hmi", HandlePreserve, hmiTag{Unit: "kPa"})
	if len(a.Data) != 1 {
		t.Fatalf("Set replaces an entry of the same name: %d entries", len(a.Data))
	}
	var tag hmiTag
	if err := a.Get("http://example.com/hmi").Decode(&tag); err != nil || tag.Unit != "kPa" {
		t.Errorf("got %+v %v", tag, err)
	}
	if _, err := a.Set("bad", HandlePreserve, make(chan int)); err == nil {
		t.Error("a value XML cannot hold")
	}
	data, err := xml.Marshal(AddDataEntry{Name: "n"})
	if err != nil || !strings.Contains(string(data), `handleUnknown="implementation"`) {
		t.Errorf("handleUnknown defaults to implementation, as TC6 requires one: %s %v", data, err)
	}
}
