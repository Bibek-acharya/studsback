import sys

file_path = '/home/durgesh/Work/studsphere/studsback/internal/scholarshipprovider/service.go'

with open(file_path, 'r') as f:
    content = f.read()

old_line = '	existing, err := s.repo.FindPublicScholarshipByProviderScholarshipID(scholarship.ID)'
new_line = '''	log.Printf("scholarshipprovider: syncPublicScholarship - syncing providerScholarshipID=%d", scholarship.ID)
	existing, err := s.repo.FindPublicScholarshipByProviderScholarshipID(scholarship.ID)'''

if old_line in content:
    content = content.replace(old_line, new_line)
elif old_line.replace('    ', '\t') in content:
    content = content.replace(old_line.replace('    ', '\t'), new_line.replace('    ', '\t'))

old_if = '	if err == nil && existing != nil {'
new_if = '''	if err == nil && existing != nil {
		log.Printf("scholarshipprovider: syncPublicScholarship - updating existing public scholarship ID=%d", existing.ID)'''

if old_if in content:
    content = content.replace(old_if, new_if)
elif old_if.replace('    ', '\t') in content:
    content = content.replace(old_if.replace('    ', '\t'), new_if.replace('    ', '\t'))

old_create = '	return s.repo.CreatePublicScholarship(publicScholarship, scholarship.ID)'
new_create = '''	log.Printf("scholarshipprovider: syncPublicScholarship - creating new public scholarship")
	return s.repo.CreatePublicScholarship(publicScholarship, scholarship.ID)'''

if old_create in content:
    content = content.replace(old_create, new_create)
elif old_create.replace('    ', '\t') in content:
    content = content.replace(old_create.replace('    ', '\t'), new_create.replace('    ', '\t'))

with open(file_path, 'w') as f:
    f.write(content)
