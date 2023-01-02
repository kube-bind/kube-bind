/*
Copyright AppsCode Inc. and Contributors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

func (section *MenuSection) Contains(rd *ResourceDescriptor) bool {
	for _, entry := range section.Items {
		if entry.Resource != nil &&
			entry.Resource.Group == rd.Spec.Resource.Group &&
			entry.Resource.Version == rd.Spec.Resource.Version &&
			entry.Resource.Name == rd.Spec.Resource.Name {
			return true
		}
	}
	return false
}

func (e MenuItem) Equals(other MenuItem) bool {
	if e.Resource != nil && other.Resource != nil {
		return *e.Resource == *other.Resource
	} else if e.Resource == nil && other.Resource == nil {
		return e.Path == other.Path
	}
	return false
}

func (a *Menu) Minus(b *Menu) {
	for _, bs := range b.Spec.Sections {
	NEXT_ENTRY:
		for _, be := range bs.Items {
			for _, as := range a.Spec.Sections {
				for idx, ae := range as.Items {
					if ae.Equals(be) {
						as.Items = append(as.Items[:idx], as.Items[idx+1:]...)
						continue NEXT_ENTRY
					}
				}
			}
		}
	}
}

func (in *MenuSectionOutlineInfo) ToMenuSectionInfo() *MenuSectionInfo {
	if in == nil {
		return nil
	}
	return &MenuSectionInfo{
		Name:       in.Name,
		Path:       in.Path,
		APIGroup:   in.AutoDiscoverAPIGroup,
		LayoutName: in.LayoutName,
		Icons:      in.Icons,
	}
}
