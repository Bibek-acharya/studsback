import sys

file_path = '/home/durgesh/Work/studsphere/studsback/internal/scholarshipprovider/service.go'

with open(file_path, 'r') as f:
    content = f.read()

# For CreateScholarship
old_line_create = '		BannerBackgroundImageURL: req.BannerBackgroundImageURL,'
new_line_create = '''		ImageURL:                 req.BannerBackgroundImageURL,
		BannerBackgroundImageURL: req.BannerBackgroundImageURL,'''

if old_line_create in content:
    content = content.replace(old_line_create, new_line_create)
elif old_line_create.replace('    ', '\t') in content:
    content = content.replace(old_line_create.replace('    ', '\t'), new_line_create.replace('    ', '\t'))

# For UpdateScholarship
old_line_update = '	updates["banner_background_image_url"] = req.BannerBackgroundImageURL'
new_line_update = '''	updates["image_url"] = req.BannerBackgroundImageURL
	updates["banner_background_image_url"] = req.BannerBackgroundImageURL'''

if old_line_update in content:
    content = content.replace(old_line_update, new_line_update)
elif old_line_update.replace('    ', '\t') in content:
    content = content.replace(old_line_update.replace('    ', '\t'), new_line_update.replace('    ', '\t'))

with open(file_path, 'w') as f:
    f.write(content)
