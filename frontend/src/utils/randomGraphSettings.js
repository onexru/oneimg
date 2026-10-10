import { createDialogScope } from './dialogScope.js';
import { selectorPage } from './renderBounds.js';
import { readApiResponse } from './apiFeedback.js';
import Message from './message.js';
import { escapeHtml } from './html.js';
export function createRandomGraphSettings() {
let disposed = false;
// 配置随机图
const { Dialog: PopupModal, dispose } = createDialogScope();
const generateRandomGraph = async () => {
    // 获取随机图配置
    try {
        const randomGraph = await getRandomGraph();
        // 打开配置弹窗
        if (!disposed) RandomGraphModal(randomGraph);
    } catch (err) {
        Message.error(err.message || '获取随机图配置失败')
    }
}

const RandomGraphModal = (randomGraph) => {
    const random_graph = randomGraph.random_graph || {}
    const user_ids = Array.isArray(randomGraph.user_ids) ? randomGraph.user_ids : []
    const tag_ids = Array.isArray(randomGraph.tag_ids) ? [...randomGraph.tag_ids] : []

    // 添加默认标签选项
    tag_ids.unshift({
        id: 0,
        name: '默认标签',
    })

    // 选中标签
    const selectedTags = [...(random_graph.tag_ids || [])]
    // 选中用户
    const selectedUsers = [...(random_graph.user_ids || [])]

    let tagPage = 1; let userPage = 1;
    const pageControls = (kind, page) => `<div class="flex items-center justify-between gap-2 mt-2"><button type="button" class="soft-button px-2 py-1 text-xs" data-selector-kind="${kind}" data-selector-page="${page.page - 1}" ${page.page <= 1 ? 'disabled' : ''}>上一页</button><span class="text-xs text-secondary">${page.page} / ${page.pages}</span><button type="button" class="soft-button px-2 py-1 text-xs" data-selector-kind="${kind}" data-selector-page="${page.page + 1}" ${page.page >= page.pages ? 'disabled' : ''}>下一页</button></div>`;
    const renderSelectors = () => {
    // 渲染标签
    const tagsHtml = selectorPage(tag_ids, tagPage).items.map(tag => `
        <div class="mb-4 last:mb-0">
            <div data-id="${Number(tag.id)}" data-type="tag" class="card-item flex items-center gap-1.5 border rounded-lg px-2.5 py-1.5 cursor-pointer transition-all duration-200 select-none ${selectedTags.includes(tag.id) ? 'border-emerald-500/50 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-400' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}">
                <i class="${selectedTags.includes(tag.id) ? 'ri-checkbox-circle-fill text-emerald-500' : 'ri-checkbox-blank-circle-line text-slate-300 dark:text-slate-600'} text-sm icon-node"></i>
                <span class="text-xs">${escapeHtml(tag.name)}</span>
            </div>
        </div>
    `).join('')

    // 渲染用户
    const usersHtml = selectorPage(user_ids, userPage).items.map(user => `
        <div class="mb-4 last:mb-0">
            <div data-id="${Number(user.id)}" data-type="user" class="card-item flex items-center gap-1.5 border rounded-lg px-2.5 py-1.5 cursor-pointer transition-all duration-200 select-none ${selectedUsers.includes(user.id) ? 'border-emerald-500/50 bg-emerald-50 dark:bg-emerald-900/20 text-emerald-700 dark:text-emerald-400' : 'border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300'}">
                <i class="${selectedUsers.includes(user.id) ? 'ri-checkbox-circle-fill text-emerald-500' : 'ri-checkbox-blank-circle-line text-slate-300 dark:text-slate-600'} text-sm icon-node"></i>
                <span class="text-xs">${escapeHtml(user.username)}</span>
            </div>
        </div>
    `).join('')

      return { tags: tagsHtml + pageControls('tag', selectorPage(tag_ids, tagPage)), users: usersHtml + pageControls('user', selectorPage(user_ids, userPage)) };
    };
    const initial = renderSelectors();

    const modalContent = `
        <div id="rg-modal-wrap" class="py-1 space-y-6 custom-scrollbar pr-2">
            <p class="text-sm text-slate-600 dark:text-slate-300">
                选择允许访问随机图的用户与标签范围，未选择时默认不限制。
            </p>
            <div>
                <h4 class="text-sm font-medium text-slate-900 dark:text-white mb-3 flex items-center gap-2">
                    <i class="ri-price-tag-3-line text-blue-500"></i> 随机图允许访问的标签范围
                </h4>
                <div id="tagCardWrap" class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-white/5">
                    ${initial.tags}
                </div>
            </div>
            <div>
                <h4 class="text-sm font-medium text-slate-900 dark:text-white mb-3 flex items-center gap-2">
                    <i class="ri-user-settings-line text-blue-500"></i> 随机图允许访问的用户范围
                </h4>
                <div id="userCardWrap" class="p-3 bg-slate-50 dark:bg-slate-800/50 rounded-xl border border-slate-100 dark:border-white/5">
                    ${initial.users}
                </div>
            </div>
        </div>
    `;

    const modal = new PopupModal({
        title: '配置随机图',
        width: '680px',
        content: modalContent,
        buttons: [
            {
                text: '取消',
                type: 'default',
                callback: () => modal.close()
            },
            {
                text: '确认保存',
                type: 'primary',
                callback: async () => {
                    try {
                        const res = await fetch(`/api/settings/randomGraph`, {
                            method: 'POST',
                            headers: {
                                'Content-Type': 'application/json',
                            },
                            body: JSON.stringify({
                                id: random_graph.id ? 1 : 0,
                                user_ids: selectedUsers,
                                tag_ids: selectedTags,
                            })
                        })
                        const data = await readApiResponse(res, '随机图配置更新失败')
                        if (data.code === 200) {
                            modal.close()
                            Message.success('随机图配置更新成功')
                        } else {
                            Message.error(data.message || '随机图配置更新失败')
                        }
                    } catch (err) {
                        Message.error('网络请求异常')
                    }
                }
            }
        ]
    })
    modal.open()

    {
        const wrap = modal.content.querySelector('#rg-modal-wrap');
        if (!wrap) return;

        wrap.addEventListener('click', (e) => {
            const control = e.target.closest('[data-selector-page]');
            if (control) {
                const page = Number(control.dataset.selectorPage);
                if (control.dataset.selectorKind === 'tag') tagPage = page; else userPage = page;
                const next = renderSelectors();
                wrap.querySelector('#tagCardWrap').innerHTML = next.tags;
                wrap.querySelector('#userCardWrap').innerHTML = next.users;
                return;
            }
            const card = e.target.closest('.card-item');
            if (!card) return;

            const id = Number(card.dataset.id);
            const type = card.dataset.type;

            const arr = type === 'tag' ? selectedTags : selectedUsers;
            const idx = arr.indexOf(id);
            const isSelected = idx > -1;

            if (isSelected) {
                arr.splice(idx, 1);
            } else {
                arr.push(id);
            }

            toggleCardUI(card, isSelected);
        });
    }

    function toggleCardUI(card, isSelected) {
        const icon = card.querySelector('.icon-node');

        const activeClasses = [
            'border-emerald-500/50', 'bg-emerald-50', 'dark:bg-emerald-900/20',
            'text-emerald-700', 'dark:text-emerald-400'
        ];
        const inactiveClasses = [
            'border-slate-200', 'dark:border-slate-700', 'bg-white',
            'dark:bg-slate-800', 'text-slate-600', 'dark:text-slate-300'
        ];

        if (isSelected) {
            card.classList.remove(...activeClasses);
            card.classList.add(...inactiveClasses);
            icon.className = 'ri-checkbox-blank-circle-line text-slate-300 dark:text-slate-600 text-sm icon-node';
        } else {
            card.classList.remove(...inactiveClasses);
            card.classList.add(...activeClasses);
            icon.className = 'ri-checkbox-circle-fill text-emerald-500 text-sm icon-node';
        }
    }
}

const getRandomGraph = async () => {
    try {
        const response = await fetch('/api/settings/randomGraph', {
            method: 'GET',
            headers: { 'Content-Type': 'application/json' }
        })
        const res = await readApiResponse(response, '获取随机图配置失败')
        if (response.ok && res.code === 200) {
            return res.data || {
                random_graph: [],
                user_ids: [],
                tag_ids: [],
            }
        } else {
            throw new Error(res.message || '获取随机图配置失败')
        }
    } catch (err) {
        throw err
    }
}


return Object.assign(generateRandomGraph, { dispose: () => { disposed = true; dispose(); } });
}
