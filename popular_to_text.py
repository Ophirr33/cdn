import re

path_re = re.compile('<td><a href="([^"]+)" title')
pop_re = re.compile('<td align="right">([0-9,]+)</')

with open('popular_raw.html', 'r') as fr:
    with open('popular.txt', 'w') as fw:
        for l in fr:
            m = path_re.match(l)
            if m:
                fw.write(m.groups()[0] + ' ')
            else:
                m = pop_re.match(l)
                if m:
                    fw.write(m.groups()[0].replace(',', '') + '\n')
